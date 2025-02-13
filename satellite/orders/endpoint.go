// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"context"
	"io"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
)

// DB implements saving order after receiving from storage node
type DB interface {
	// CreateSerialInfo creates serial number entry in database
	CreateSerialInfo(ctx context.Context, serialNumber storj.SerialNumber, bucketID []byte, limitExpiration time.Time) error
	// UseSerialNumber creates serial number entry in database
	UseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) ([]byte, error)
	// UnuseSerialNumber removes pair serial number -> storage node id from database
	UnuseSerialNumber(ctx context.Context, serialNumber storj.SerialNumber, storageNodeID storj.NodeID) error
	// DeleteExpiredSerials deletes all expired serials in serial_number and used_serials table.
	DeleteExpiredSerials(ctx context.Context, now time.Time) (_ int, err error)

	// UpdateBucketBandwidthAllocation updates 'allocated' bandwidth for given bucket
	UpdateBucketBandwidthAllocation(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthSettle updates 'settled' bandwidth for given bucket
	UpdateBucketBandwidthSettle(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateBucketBandwidthInline updates 'inline' bandwidth for given bucket
	UpdateBucketBandwidthInline(ctx context.Context, projectID uuid.UUID, bucketName []byte, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// UpdateStoragenodeBandwidthAllocation updates 'allocated' bandwidth for given storage nodes
	UpdateStoragenodeBandwidthAllocation(ctx context.Context, storageNodes []storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error
	// UpdateStoragenodeBandwidthSettle updates 'settled' bandwidth for given storage node
	UpdateStoragenodeBandwidthSettle(ctx context.Context, storageNode storj.NodeID, action pb.PieceAction, amount int64, intervalStart time.Time) error

	// GetBucketBandwidth gets total bucket bandwidth from period of time
	GetBucketBandwidth(ctx context.Context, projectID uuid.UUID, bucketName []byte, from, to time.Time) (int64, error)
	// GetStorageNodeBandwidth gets total storage node bandwidth from period of time
	GetStorageNodeBandwidth(ctx context.Context, nodeID storj.NodeID, from, to time.Time) (int64, error)

	// ProcessOrders takes a list of order requests and processes them in a batch
	ProcessOrders(ctx context.Context, requests []*ProcessOrderRequest) (responses []*ProcessOrderResponse, err error)
}

var (
	// Error the default orders errs class
	Error = errs.Class("orders error")
	// ErrUsingSerialNumber error class for serial number
	ErrUsingSerialNumber = errs.Class("serial number")

	mon = monkit.Package()
)

// ProcessOrderRequest for batch order processing
type ProcessOrderRequest struct {
	Order      *pb.Order
	OrderLimit *pb.OrderLimit
}

// ProcessOrderResponse for batch order processing responses
type ProcessOrderResponse struct {
	SerialNumber storj.SerialNumber
	Status       pb.SettlementResponse_Status
}

// Endpoint for orders receiving
type Endpoint struct {
	log                 *zap.Logger
	satelliteSignee     signing.Signee
	DB                  DB
	settlementBatchSize int
}

// NewEndpoint new orders receiving endpoint
func NewEndpoint(log *zap.Logger, satelliteSignee signing.Signee, db DB, settlementBatchSize int) *Endpoint {
	return &Endpoint{
		log:                 log,
		satelliteSignee:     satelliteSignee,
		DB:                  db,
		settlementBatchSize: settlementBatchSize,
	}
}

func monitoredSettlementStreamReceive(ctx context.Context, stream pb.Orders_SettlementServer) (_ *pb.SettlementRequest, err error) {
	defer mon.Task()(&ctx)(&err)
	return stream.Recv()
}

func monitoredSettlementStreamSend(ctx context.Context, stream pb.Orders_SettlementServer, resp *pb.SettlementResponse) (err error) {
	defer mon.Task()(&ctx)(&err)
	switch resp.Status {
	case pb.SettlementResponse_ACCEPTED:
		mon.Event("settlement_response_accepted")
	case pb.SettlementResponse_REJECTED:
		mon.Event("settlement_response_rejected")
	default:
		mon.Event("settlement_response_unknown")
	}
	return stream.Send(resp)
}

// Settlement receives orders and handles them in batches
func (endpoint *Endpoint) Settlement(stream pb.Orders_SettlementServer) (err error) {
	ctx := stream.Context()
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	formatError := func(err error) error {
		if err == io.EOF {
			return nil
		}
		return status.Error(codes.Unknown, err.Error())
	}

	log := endpoint.log.Named(peer.ID.String())
	log.Debug("Settlement")

	requests := make([]*ProcessOrderRequest, 0, endpoint.settlementBatchSize)

	defer func() {
		if len(requests) > 0 {
			err = errs.Combine(err, endpoint.processOrders(ctx, stream, requests))
			if err != nil {
				err = formatError(err)
			}
		}
	}()

	for {
		request, err := monitoredSettlementStreamReceive(ctx, stream)
		if err != nil {
			return formatError(err)
		}

		if request == nil {
			return status.Error(codes.InvalidArgument, "request missing")
		}
		if request.Limit == nil {
			return status.Error(codes.InvalidArgument, "order limit missing")
		}
		if request.Order == nil {
			return status.Error(codes.InvalidArgument, "order missing")
		}

		orderLimit := request.Limit
		order := request.Order

		if orderLimit.StorageNodeId != peer.ID {
			return status.Error(codes.Unauthenticated, "only specified storage node can settle order")
		}

		rejectErr := func() error {
			// satellite verifies that it signed the order limit
			if err := signing.VerifyOrderLimitSignature(ctx, endpoint.satelliteSignee, orderLimit); err != nil {
				return Error.New("unable to verify order limit")
			}

			// satellite verifies that the order signature matches pub key in order limit
			if err := signing.VerifyUplinkOrderSignature(ctx, orderLimit.UplinkPublicKey, order); err != nil {
				return Error.New("unable to verify order")
			}

			// TODO should this reject or just error ??
			if orderLimit.SerialNumber != order.SerialNumber {
				return Error.New("invalid serial number")
			}

			if orderLimit.OrderExpiration.Before(time.Now()) {
				return Error.New("order limit expired")
			}
			return nil
		}()
		if rejectErr != nil {
			log.Debug("order limit/order verification failed", zap.Stringer("serial", orderLimit.SerialNumber), zap.Error(rejectErr))
			err := monitoredSettlementStreamSend(ctx, stream, &pb.SettlementResponse{
				SerialNumber: orderLimit.SerialNumber,
				Status:       pb.SettlementResponse_REJECTED,
			})
			if err != nil {
				return formatError(err)
			}
			continue
		}

		requests = append(requests, &ProcessOrderRequest{Order: order, OrderLimit: orderLimit})

		if len(requests) >= endpoint.settlementBatchSize {
			err = endpoint.processOrders(ctx, stream, requests)
			requests = requests[:0]
			if err != nil {
				return formatError(err)
			}
		}
	}
}

func (endpoint *Endpoint) processOrders(ctx context.Context, stream pb.Orders_SettlementServer, requests []*ProcessOrderRequest) (err error) {
	defer mon.Task()(&ctx)(&err)

	responses, err := endpoint.DB.ProcessOrders(ctx, requests)
	if err != nil {
		return err
	}

	for _, response := range responses {
		r := &pb.SettlementResponse{
			SerialNumber: response.SerialNumber,
			Status:       response.Status,
		}
		err = monitoredSettlementStreamSend(ctx, stream, r)
		if err != nil {
			return err
		}
	}
	return nil
}
