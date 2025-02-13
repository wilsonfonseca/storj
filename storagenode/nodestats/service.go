// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	// NodeStatsServiceErr defines node stats service error
	NodeStatsServiceErr = errs.Class("node stats service error")

	mon = monkit.Package()
)

// Client encapsulates NodeStatsClient with underlying connection
type Client struct {
	conn *grpc.ClientConn
	pb.NodeStatsClient
}

// Close closes underlying client connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// Service retrieves info from satellites using GRPC client
type Service struct {
	log *zap.Logger

	transport transport.Client
	trust     *trust.Pool
}

// NewService creates new instance of service
func NewService(log *zap.Logger, transport transport.Client, trust *trust.Pool) *Service {
	return &Service{
		log:       log,
		transport: transport,
		trust:     trust,
	}
}

// GetReputationStats retrieves reputation stats from particular satellite
func (s *Service) GetReputationStats(ctx context.Context, satelliteID storj.NodeID) (_ *reputation.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.dial(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	defer func() {
		if cerr := client.Close(); cerr != nil {
			err = errs.Combine(err, NodeStatsServiceErr.New("failed to close connection: %v", cerr))
		}
	}()

	resp, err := client.GetStats(ctx, &pb.GetStatsRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	uptime := resp.GetUptimeCheck()
	audit := resp.GetAuditCheck()

	return &reputation.Stats{
		SatelliteID: satelliteID,
		Uptime: reputation.Metric{
			TotalCount:   uptime.GetTotalCount(),
			SuccessCount: uptime.GetSuccessCount(),
			Alpha:        uptime.GetReputationAlpha(),
			Beta:         uptime.GetReputationBeta(),
			Score:        uptime.GetReputationScore(),
		},
		Audit: reputation.Metric{
			TotalCount:   audit.GetTotalCount(),
			SuccessCount: audit.GetSuccessCount(),
			Alpha:        audit.GetReputationAlpha(),
			Beta:         audit.GetReputationBeta(),
			Score:        audit.GetReputationScore(),
		},
		Disqualified: resp.GetDisqualified(),
		UpdatedAt:    time.Now(),
	}, nil
}

// GetDailyStorageUsage returns daily storage usage over a period of time for a particular satellite
func (s *Service) GetDailyStorageUsage(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.dial(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	defer func() {
		if cerr := client.Close(); cerr != nil {
			err = errs.Combine(err, NodeStatsServiceErr.New("failed to close connection: %v", cerr))
		}
	}()

	resp, err := client.DailyStorageUsage(ctx, &pb.DailyStorageUsageRequest{From: from, To: to})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return fromSpaceUsageResponse(resp, satelliteID), nil
}

// dial dials GRPC NodeStats client for the satellite by id
func (s *Service) dial(ctx context.Context, satelliteID storj.NodeID) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	address, err := s.trust.GetAddress(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %v", satelliteID, err)
	}

	satellite := pb.Node{
		Id: satelliteID,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   address,
		},
	}

	conn, err := s.transport.DialNode(ctx, &satellite)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %v", satelliteID, err)
	}

	return &Client{
		conn:            conn,
		NodeStatsClient: pb.NewNodeStatsClient(conn),
	}, nil
}

// fromSpaceUsageResponse get DiskSpaceUsage slice from pb.SpaceUsageResponse
func fromSpaceUsageResponse(resp *pb.DailyStorageUsageResponse, satelliteID storj.NodeID) []storageusage.Stamp {
	var stamps []storageusage.Stamp

	for _, pbUsage := range resp.GetDailyStorageUsage() {
		stamps = append(stamps, storageusage.Stamp{
			SatelliteID: satelliteID,
			AtRestTotal: pbUsage.AtRestTotal,
			Timestamp:   pbUsage.Timestamp,
		})
	}

	return stamps
}
