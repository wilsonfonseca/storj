// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"sync"
	"time"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Inmemory implements inmemory database for overlay.Cache
type Inmemory struct {
	mu     sync.RWMutex
	nodes  []*NodeInfo
	lookup map[storj.NodeID]*NodeInfo
}

// NodeInfo blah
type NodeInfo struct {
	ID storj.NodeID

	Address       string
	LastNet       string
	Protocol      pb.NodeTransport
	Type          pb.NodeType
	Email         string
	Wallet        string
	FreeBandwidth int64
	FreeDisk      int64

	LastContactSuccess time.Time
	LastContactFailure time.Time

	AuditSuccessCount  int64
	TotalAuditCount    int64
	UptimeSuccessCount int64
	TotalUptimeCount   int64

	AuditReputationAlpha  float64
	AuditReputationBeta   float64
	UptimeReputationAlpha float64
	UptimeReputationBeta  float64

	Contained bool

	CreatedAt    time.Time
	UpdatedAt    time.Time
	Disqualified time.Time

	Version pb.NodeVersion
}

// SelectStorageNodes looks up nodes based on criteria
func (db *Inmemory) SelectStorageNodes(ctx context.Context, count int, criteria *NodeCriteria) ([]*pb.Node, error) {
	return nil, nil
}

// SelectNewStorageNodes looks up nodes based on new node criteria
func (db *Inmemory) SelectNewStorageNodes(ctx context.Context, count int, criteria *NodeCriteria) ([]*pb.Node, error) {
	return nil, nil
}

// Get looks up the node by nodeID
func (db *Inmemory) Get(ctx context.Context, nodeID storj.NodeID) (*NodeDossier, error) {
	return nil, nil
}

// KnownOffline filters a set of nodes to offline nodes
func (db *Inmemory) KnownOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error) {
	return nil, nil
}

// KnownUnreliableOrOffline filters a set of nodes to unhealth or offlines node, independent of new
func (db *Inmemory) KnownUnreliableOrOffline(context.Context, *NodeCriteria, storj.NodeIDList) (storj.NodeIDList, error) {
	return nil, nil
}

// Reliable returns all nodes that are reliable
func (db *Inmemory) Reliable(context.Context, *NodeCriteria) (storj.NodeIDList, error) {
	return nil, nil
}

// Paginate will page through the database nodes
func (db *Inmemory) Paginate(ctx context.Context, offset int64, limit int) ([]*NodeDossier, bool, error) {
	return nil, nil
}

// IsVetted returns whether or not the node reaches reputable thresholds
func (db *Inmemory) IsVetted(ctx context.Context, id storj.NodeID, criteria *NodeCriteria) (bool, error) {
	return nil, nil
}

// UpdateAddress updates node address
func (db *Inmemory) UpdateAddress(ctx context.Context, value *pb.Node, defaults NodeSelectionConfig) error {
	return nil, nil
}

// UpdateStats all parts of single storagenode's stats.
func (db *Inmemory) UpdateStats(ctx context.Context, request *UpdateRequest) (stats *NodeStats, err error) {
	return nil, nil
}

// UpdateNodeInfo updates node dossier with info requested from the node itself like node type, email, wallet, capacity, and version.
func (db *Inmemory) UpdateNodeInfo(ctx context.Context, node storj.NodeID, nodeInfo *pb.InfoResponse) (stats *NodeDossier, err error) {
	return nil, nil
}

// UpdateUptime updates a single storagenode's uptime stats.
func (db *Inmemory) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool, lambda, weight, uptimeDQ float64) (stats *NodeStats, err error) {
	return nil, nil
}
