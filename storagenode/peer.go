// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/console/consoleserver"
	"storj.io/storj/storagenode/inspector"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/nodestats"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()
)

// DB is the master database for Storage Node
type DB interface {
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	Pieces() storage.Blobs

	Orders() orders.DB
	V0PieceInfo() pieces.V0PieceInfoDB
	PieceExpirationDB() pieces.PieceExpirationDB
	PieceSpaceUsedDB() pieces.PieceSpaceUsedDB
	Bandwidth() bandwidth.DB
	UsedSerials() piecestore.UsedSerials
	Console() console.DB
	Reputation() reputation.DB
	StorageUsage() storageusage.DB

	// TODO: use better interfaces
	RoutingTable() (kdb, ndb, adb storage.KeyValueStore)
}

// Config is all the configuration parameters for a Storage Node
type Config struct {
	Identity identity.Config

	Server   server.Config
	Kademlia kademlia.Config

	// TODO: flatten storage config and only keep the new one
	Storage   piecestore.OldConfig
	Storage2  piecestore.Config
	Collector collector.Config

	Retain retain.Config

	Nodestats nodestats.Config

	Console consoleserver.Config

	Version version.Config

	Bandwidth bandwidth.Config
}

// Verify verifies whether configuration is consistent and acceptable.
func (config *Config) Verify(log *zap.Logger) error {
	return config.Kademlia.Verify(log)
}

// Peer is the representation of a Storage Node.
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity
	DB       DB

	Transport transport.Client

	Server *server.Server

	Version *version.Service

	// services and endpoints
	// TODO: similar grouping to satellite.Peer
	Kademlia struct {
		RoutingTable *kademlia.RoutingTable
		Service      *kademlia.Kademlia
		Endpoint     *kademlia.Endpoint
		Inspector    *kademlia.Inspector
	}

	Storage2 struct {
		// TODO: lift things outside of it to organize better
		Trust         *trust.Pool
		Store         *pieces.Store
		BlobsCache    *pieces.BlobsUsageCache
		CacheService  *pieces.CacheService
		RetainService *retain.Service
		Endpoint      *piecestore.Endpoint
		Inspector     *inspector.Endpoint
		Monitor       *monitor.Service
		Orders        *orders.Service
	}

	Collector *collector.Service

	NodeStats struct {
		Service *nodestats.Service
		Cache   *nodestats.Cache
	}

	// Web server with web UI
	Console struct {
		Listener net.Listener
		Service  *console.Service
		Endpoint *consoleserver.Server
	}

	Bandwidth *bandwidth.Service
}

// New creates a new Storage Node.
func New(log *zap.Logger, full *identity.FullIdentity, db DB, revocationDB extensions.RevocationDB, config Config, versionInfo version.Info) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: full,
		DB:       db,
	}

	var err error

	{
		test := version.Info{}
		if test != versionInfo {
			peer.Log.Sugar().Debugf("Binary Version: %s with CommitHash %s, built at %s as Release %v",
				versionInfo.Version.String(), versionInfo.CommitHash, versionInfo.Timestamp.String(), versionInfo.Release)
		}
		peer.Version = version.NewService(log.Named("version"), config.Version, versionInfo, "Storagenode")
	}

	{ // setup listener and server
		sc := config.Server

		options, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Transport = transport.NewClient(options)

		peer.Server, err = server.New(log.Named("server"), options, sc.Address, sc.PrivateAddress, nil)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup trust pool before kademlia
		peer.Storage2.Trust, err = trust.NewPool(peer.Transport, config.Storage.WhitelistedSatellites)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
	}

	{ // setup kademlia
		config := config.Kademlia
		// TODO: move this setup logic into kademlia package
		if config.ExternalAddress == "" {
			config.ExternalAddress = peer.Addr()
		}

		pbVersion, err := versionInfo.Proto()
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		self := &overlay.NodeDossier{
			Node: pb.Node{
				Id: peer.ID(),
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   config.ExternalAddress,
				},
			},
			Type: pb.NodeType_STORAGE,
			Operator: pb.NodeOperator{
				Email:  config.Operator.Email,
				Wallet: config.Operator.Wallet,
			},
			Version: *pbVersion,
		}

		kdb, ndb, adb := peer.DB.RoutingTable()
		peer.Kademlia.RoutingTable, err = kademlia.NewRoutingTable(peer.Log.Named("routing"), self, kdb, ndb, adb, &config.RoutingTableConfig)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Transport = peer.Transport.WithObservers(peer.Kademlia.RoutingTable)

		peer.Kademlia.Service, err = kademlia.NewService(peer.Log.Named("kademlia"), peer.Transport, peer.Kademlia.RoutingTable, config)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Kademlia.Endpoint = kademlia.NewEndpoint(peer.Log.Named("kademlia:endpoint"), peer.Kademlia.Service, peer.Kademlia.RoutingTable, peer.Storage2.Trust)
		pb.RegisterNodesServer(peer.Server.GRPC(), peer.Kademlia.Endpoint)

		peer.Kademlia.Inspector = kademlia.NewInspector(peer.Kademlia.Service, peer.Identity)
		pb.RegisterKadInspectorServer(peer.Server.PrivateGRPC(), peer.Kademlia.Inspector)
	}

	{ // setup storage
		peer.Storage2.BlobsCache = pieces.NewBlobsUsageCache(peer.DB.Pieces())

		peer.Storage2.Store = pieces.NewStore(peer.Log.Named("pieces"),
			peer.Storage2.BlobsCache,
			peer.DB.V0PieceInfo(),
			peer.DB.PieceExpirationDB(),
			peer.DB.PieceSpaceUsedDB(),
		)

		peer.Storage2.CacheService = pieces.NewService(
			log.Named("piecestore:cacheUpdate"),
			peer.Storage2.BlobsCache,
			peer.Storage2.Store,
			config.Storage2.CacheSyncInterval,
		)

		peer.Storage2.Monitor = monitor.NewService(
			log.Named("piecestore:monitor"),
			peer.Kademlia.RoutingTable,
			peer.Storage2.Store,
			peer.DB.Bandwidth(),
			config.Storage.AllocatedDiskSpace.Int64(),
			config.Storage.AllocatedBandwidth.Int64(),
			//TODO use config.Storage.Monitor.Interval, but for some reason is not set
			config.Storage.KBucketRefreshInterval,
			config.Storage2.Monitor,
		)

		peer.Storage2.RetainService = retain.NewService(
			peer.Log.Named("retain"),
			peer.Storage2.Store,
			config.Retain,
		)

		peer.Storage2.Endpoint, err = piecestore.NewEndpoint(
			peer.Log.Named("piecestore"),
			signing.SignerFromFullIdentity(peer.Identity),
			peer.Storage2.Trust,
			peer.Storage2.Monitor,
			peer.Storage2.RetainService,
			peer.Storage2.Store,
			peer.DB.Orders(),
			peer.DB.Bandwidth(),
			peer.DB.UsedSerials(),
			config.Storage2,
		)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		pb.RegisterPiecestoreServer(peer.Server.GRPC(), peer.Storage2.Endpoint)

		sc := config.Server
		options, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		// TODO workaround for custom timeout for order sending request (read/write)
		ordersTransport := transport.NewClientWithTimeouts(options, transport.Timeouts{
			Dial:    config.Storage2.Orders.SenderDialTimeout,
			Request: config.Storage2.Orders.SenderRequestTimeout,
		})

		peer.Storage2.Orders = orders.NewService(
			log.Named("orders"),
			ordersTransport,
			peer.DB.Orders(),
			peer.Storage2.Trust,
			config.Storage2.Orders,
		)
	}

	{ // setup node stats service
		peer.NodeStats.Service = nodestats.NewService(
			peer.Log.Named("nodestats:service"),
			peer.Transport,
			peer.Storage2.Trust)

		peer.NodeStats.Cache = nodestats.NewCache(
			peer.Log.Named("nodestats:cache"),
			config.Nodestats,
			nodestats.CacheStorage{
				Reputation:   peer.DB.Reputation(),
				StorageUsage: peer.DB.StorageUsage(),
			},
			peer.NodeStats.Service,
			peer.Storage2.Trust)
	}

	{ // setup storage node operator dashboard
		peer.Console.Service, err = console.NewService(
			peer.Log.Named("console:service"),
			peer.DB.Console(),
			peer.DB.Bandwidth(),
			peer.Storage2.Store,
			peer.Kademlia.Service,
			peer.Version,
			config.Storage.AllocatedBandwidth,
			config.Storage.AllocatedDiskSpace,
			config.Kademlia.Operator.Wallet,
			versionInfo,
			peer.Storage2.Trust,
			peer.DB.Reputation(),
			peer.DB.StorageUsage())

		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Listener, err = net.Listen("tcp", config.Console.Address)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}

		peer.Console.Endpoint = consoleserver.NewServer(
			peer.Log.Named("console:endpoint"),
			config.Console,
			peer.Console.Service,
			peer.Console.Listener,
		)
	}

	{ // setup storage inspector
		peer.Storage2.Inspector = inspector.NewEndpoint(
			peer.Log.Named("pieces:inspector"),
			peer.Storage2.Store,
			peer.Kademlia.Service,
			peer.DB.Bandwidth(),
			config.Storage,
			peer.Console.Listener.Addr(),
		)
		pb.RegisterPieceStoreInspectorServer(peer.Server.PrivateGRPC(), peer.Storage2.Inspector)
	}

	peer.Collector = collector.NewService(peer.Log.Named("collector"), peer.Storage2.Store, peer.DB.UsedSerials(), config.Collector)

	peer.Bandwidth = bandwidth.NewService(peer.Log.Named("bandwidth"), peer.DB.Bandwidth(), config.Bandwidth)

	return peer, nil
}

// Run runs storage node until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Version.Run(ctx))
	})

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Kademlia.Service.Bootstrap(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Kademlia.Service.Run(ctx))
	})

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Collector.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.Orders.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.Monitor.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.CacheService.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Storage2.RetainService.Run(ctx))
	})

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Bandwidth.Run(ctx))
	})

	group.Go(func() error {
		// TODO: move the message into Server instead
		// Don't change the format of this comment, it is used to figure out the node id.
		peer.Log.Sugar().Infof("Node %s started", peer.Identity.ID)
		peer.Log.Sugar().Infof("Public server started on %s", peer.Addr())
		peer.Log.Sugar().Infof("Private server started on %s", peer.PrivateAddr())
		return errs2.IgnoreCanceled(peer.Server.Run(ctx))
	})

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.NodeStats.Cache.Run(ctx))
	})
	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Console.Endpoint.Run(ctx))
	})

	return group.Wait()
}

// Close closes all the resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	// TODO: ensure that Close can be called on nil-s that way this code won't need the checks.

	// close servers, to avoid new connections to closing subsystems
	if peer.Server != nil {
		errlist.Add(peer.Server.Close())
	}

	// close services in reverse initialization order

	if peer.Bandwidth != nil {
		errlist.Add(peer.Bandwidth.Close())
	}
	if peer.Storage2.RetainService != nil {
		errlist.Add(peer.Storage2.RetainService.Close())
	}
	if peer.Storage2.Monitor != nil {
		errlist.Add(peer.Storage2.Monitor.Close())
	}
	if peer.Storage2.Orders != nil {
		errlist.Add(peer.Storage2.Orders.Close())
	}
	if peer.Storage2.CacheService != nil {
		errlist.Add(peer.Storage2.CacheService.Close())
	}
	if peer.Collector != nil {
		errlist.Add(peer.Collector.Close())
	}

	if peer.Kademlia.Service != nil {
		errlist.Add(peer.Kademlia.Service.Close())
	}
	if peer.Kademlia.RoutingTable != nil {
		errlist.Add(peer.Kademlia.RoutingTable.Close())
	}
	if peer.Console.Endpoint != nil {
		errlist.Add(peer.Console.Endpoint.Close())
	} else if peer.Console.Listener != nil {
		errlist.Add(peer.Console.Listener.Close())
	}

	if peer.NodeStats.Cache != nil {
		errlist.Add(peer.NodeStats.Cache.Close())
	}

	return errlist.Err()
}

// ID returns the peer ID.
func (peer *Peer) ID() storj.NodeID { return peer.Identity.ID }

// Local returns the peer local node info.
func (peer *Peer) Local() overlay.NodeDossier { return peer.Kademlia.RoutingTable.Local() }

// Addr returns the public address.
func (peer *Peer) Addr() string { return peer.Server.Addr().String() }

// URL returns the storj.NodeURL.
func (peer *Peer) URL() storj.NodeURL { return storj.NodeURL{ID: peer.ID(), Address: peer.Addr()} }

// PrivateAddr returns the private address.
func (peer *Peer) PrivateAddr() string { return peer.Server.PrivateAddr().String() }
