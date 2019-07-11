// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"fmt"
	"io"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// Downloader is interface that can be used for downloading content.
// It matches signature of `io.ReadCloser`.
type Downloader interface {
	Read([]byte) (int, error)
	Close() error
}

// Download implements downloading from a piecestore.
type Download struct {
	client *Client
	limit  *pb.OrderLimit
	peer   *identity.PeerIdentity
	stream pb.Piecestore_DownloadClient
	ctx    context.Context

	read         int64 // how much data we have read so far
	allocated    int64 // how far have we sent orders
	downloaded   int64 // how much data have we downloaded
	downloadSize int64 // how much do we want to download

	// what is the step we consider to upload
	allocationStep int64

	unread ReadBuffer
}

// Download starts a new download using the specified order limit at the specified offset and size.
func (client *Client) Download(ctx context.Context, limit *pb.OrderLimit, offset, size int64) (_ Downloader, err error) {
	defer mon.Task()(&ctx)(&err)

	stream, err := client.client.Download(ctx)
	if err != nil {
		return nil, err
	}

	peer, err := identity.PeerIdentityFromContext(stream.Context())
	if err != nil {
		closeErr := stream.CloseSend()
		_, recvErr := stream.Recv()
		return nil, ErrInternal.Wrap(errs.Combine(err, ignoreEOF(ctx, closeErr), ignoreEOF(ctx, recvErr)))
	}

	err = stream.Send(&pb.PieceDownloadRequest{
		Limit: limit,
		Chunk: &pb.PieceDownloadRequest_Chunk{
			Offset:    offset,
			ChunkSize: size,
		},
	})
	if err != nil {
		_, recvErr := stream.Recv()
		return nil, ErrProtocol.Wrap(errs.Combine(ignoreEOF(ctx, err), ignoreEOF(ctx, recvErr)))
	}

	download := &Download{
		client: client,
		limit:  limit,
		peer:   peer,
		stream: stream,
		ctx:    ctx,

		read: 0,

		allocated:    0,
		downloaded:   0,
		downloadSize: size,

		allocationStep: client.config.InitialStep,
	}

	if client.config.DownloadBufferSize <= 0 {
		return &LockingDownload{download: download}, nil
	}
	return &LockingDownload{
		download: NewBufferedDownload(download, int(client.config.DownloadBufferSize)),
	}, nil
}

// Read downloads data from the storage node allocating as necessary.
func (client *Download) Read(data []byte) (read int, err error) {
	ctx := client.ctx
	defer mon.Task()(&ctx)(&err)
	for client.read < client.downloadSize {
		// read from buffer
		n, err := client.unread.Read(data)
		client.read += int64(n)
		read += n

		// if we have an error or are pending for an error, avoid further communication
		// however we should still finish reading the unread data.
		if err != nil || client.unread.Errored() {
			return read, err
		}

		// do we need to send a new order to storagenode
		if client.allocated-client.downloaded < client.allocationStep {
			newAllocation := client.allocationStep

			// have we downloaded more than we have allocated due to a generous storagenode?
			if client.allocated-client.downloaded < 0 {
				newAllocation += client.downloaded - client.allocated
			}

			// ensure we don't allocate more than we intend to read
			if client.allocated+newAllocation > client.downloadSize {
				newAllocation = client.downloadSize - client.allocated
			}

			// send an order
			if newAllocation > 0 {
				// sign the order
				order, err := signing.SignOrder(ctx, client.client.signer, &pb.Order{
					SerialNumber: client.limit.SerialNumber,
					Amount:       newAllocation,
				})
				// something went wrong with signing
				if err != nil {
					client.unread.IncludeError(err)
					return read, nil
				}

				err = client.stream.Send(&pb.PieceDownloadRequest{
					Order: order,
				})
				if err != nil {
					// other side doesn't want to talk to us anymore,
					// or network went down
					client.unread.IncludeError(err)
					return read, nil
				}

				// update our allocation step
				client.allocationStep = client.client.nextAllocationStep(client.allocationStep)
			}
		} // if end allocation sending

		// we have data, no need to wait for a chunk
		if read > 0 {
			return read, nil
		}

		// we don't have data, wait for a chunk from storage node
		response, err := client.stream.Recv()
		if response != nil && response.Chunk != nil {
			client.downloaded += int64(len(response.Chunk.Data))
			client.unread.Fill(response.Chunk.Data)
		}

		// we still need to continue until we have actually handled all of the errors
		client.unread.IncludeError(err)
	}

	// all downloaded
	if read == 0 {
		return 0, io.EOF
	}
	return read, nil
}

// Close closes the downloading.
func (client *Download) Close() (err error) {
	defer func() {
		if err != nil {
			details := errs.Class(fmt.Sprintf("(Node ID: %s, Piece ID: %s)", client.peer.ID.String(), client.limit.PieceId.String()))
			err = details.Wrap(err)
			err = Error.Wrap(err)
		}
	}()

	alldone := client.read == client.downloadSize

	// close our sending end
	closeErr := client.stream.CloseSend()
	// try to read any pending error message
	_, recvErr := client.stream.Recv()

	if alldone {
		// if we are all done, then we expecte io.EOF, but don't care about them
		return errs.Combine(ignoreEOF(client.ctx, closeErr), ignoreEOF(client.ctx, recvErr))
	}

	if client.unread.Errored() {
		// something went wrong and we didn't manage to download all the content
		return errs.Combine(client.unread.Error(), ignoreEOF(client.ctx, closeErr), ignoreEOF(client.ctx, recvErr))
	}

	// we probably closed download early, so we can ignore io.EOF-s
	return errs.Combine(ignoreEOF(client.ctx, closeErr), ignoreEOF(client.ctx, recvErr))
}

// ReadBuffer implements buffered reading with an error.
type ReadBuffer struct {
	data []byte
	err  error
}

// Error returns an error if it was encountered.
func (buffer *ReadBuffer) Error() error { return buffer.err }

// Errored returns whether the buffer contains an error.
func (buffer *ReadBuffer) Errored() bool { return buffer.err != nil }

// Empty checks whether buffer needs to be filled.
func (buffer *ReadBuffer) Empty() bool {
	return len(buffer.data) == 0 && buffer.err == nil
}

// IncludeError adds error at the end of the buffer.
func (buffer *ReadBuffer) IncludeError(err error) {
	buffer.err = errs.Combine(buffer.err, err)
}

// Fill fills the buffer with the specified bytes.
func (buffer *ReadBuffer) Fill(data []byte) {
	buffer.data = data
}

// Read reads from the buffer.
func (buffer *ReadBuffer) Read(data []byte) (n int, err error) {
	if len(buffer.data) > 0 {
		n = copy(data, buffer.data)
		buffer.data = buffer.data[n:]
		return n, nil
	}

	if buffer.err != nil {
		return 0, buffer.err
	}

	return 0, nil
}
