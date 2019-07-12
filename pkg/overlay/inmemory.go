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
	mu       sync.RWMutex
	nodes    []*NodeProtectedDossier
	lookupID map[storj.NodeID]*NodeProtectedDossier
}

// NodeProtectedDossier blah
type NodeProtectedDossier struct {
	sync.RWMutex

	ID        storj.NodeID
	Transport pb.NodeTransport
	Type      pb.NodeType

	Address string
	LastNet string

	// Operator
	Email  string
	Wallet string

	// Capacity
	FreeBandwidth int64
	FreeDisk      int64

	Reputation NodeStats

	CreatedAt time.Time
	UpdatedAt time.Time

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
	return nil, false, nil
}

// IsVetted returns whether or not the node reaches reputable thresholds
func (db *Inmemory) IsVetted(ctx context.Context, id storj.NodeID, criteria *NodeCriteria) (bool, error) {
	return false, nil
}

func (db *Inmemory) lookup(ctx context.Context, id storj.NodeID) *NodeProtectedDossier {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.lookupID[id]
}

// must hold write lock
func (db *Inmemory) _add(ctx context.Context, dossier *NodeProtectedDossier) *NodeProtectedDossier {
	db.lookupID[dossier.ID] = dossier
	// TODO: insert sorted
	db.nodes = append(db.nodes, dossier)
	return dossier
}

// UpdateAddress updates node address
func (db *Inmemory) UpdateAddress(ctx context.Context, info *pb.Node, defaults NodeSelectionConfig) error {
	if info == nil || info.Id.IsZero() {
		return ErrEmptyNode
	}

	// fast path
	if dossier := db.lookup(ctx, info.Id); dossier != nil {
		dossier.Lock()
		defer dossier.Unlock()

		dossier.Address = info.Address.Address
		dossier.Transport = info.Address.Transport
		dossier.LastNet = info.LastIp
		return nil
	}

	now := time.Now()

	db.mu.Lock()
	defer db.mu.Lock()

	db._add(ctx, &NodeProtectedDossier{
		ID:            info.Id,
		Address:       info.Address.Address,
		Transport:     info.Address.Transport,
		LastNet:       info.LastIp,
		Type:          pb.NodeType_INVALID,
		FreeBandwidth: -1,
		FreeDisk:      -1,
		Reputation: NodeStats{
			LastContactSuccess: now,

			AuditReputationAlpha:  defaults.AuditReputationAlpha0,
			AuditReputationBeta:   defaults.AuditReputationBeta0,
			UptimeReputationAlpha: defaults.UptimeReputationAlpha0,
			UptimeReputationBeta:  defaults.UptimeReputationBeta0,
		},
	})

	return nil
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
	if nodeID.IsZero() {
		return nil, ErrEmptyNode
	}

	dossier := db.lookup(ctx, nodeID)
	if dossier == nil {
		return nil, ErrNodeNotFound.New("%s", nodeID)
	}

	dossier.Lock()
	defer dossier.Unlock()

	if dossier.Reputation.Disqualified != nil {
		return dossier._nodeStats(), nil
	}

	uptimeAlpha, uptimeBeta, totalUptimeCount := updateReputation(
		isUp,
		dossier.Reputation.UptimeReputationAlpha,
		dossier.Reputation.UptimeReputationBeta,
		lambda,
		weight,
		dossier.Reputation.UptimeCount,
	)
	dossier.Reputation.UptimeReputationAlpha = uptimeAlpha
	dossier.Reputation.UptimeReputationBeta = uptimeBeta
	dossier.Reputation.UptimeCount = totalUptimeCount 

	
	uptimeRep := uptimeAlpha / (uptimeAlpha + uptimeBeta)
	if uptimeRep <= uptimeDQ {
		now := time.Now()
		dossier.Reputation.Disqualified = &now
	}
	if isUp {
		dossier.Reputation.UptimeSuccessCount++
		dossier.Reputation.LastContactSuccess = time.Now()
	} else {
		dossier.Reputation.LastContactFailure = time.Now()
	}

	return dossier._nodeStats(), nil
}

func (dossier *NodeProtectedDossier) _nodeStats() *NodeStats {
	clone := dossier.Reputation
	return &clone
}

// updateReputation uses the Beta distribution model to determine a node's reputation.
// lambda is the "forgetting factor" which determines how much past info is kept when determining current reputation score.
// w is the normalization weight that affects how severely new updates affect the current reputation distribution.
func updateReputation(isSuccess bool, alpha, beta, lambda, w float64, totalCount int64) (newAlpha, newBeta float64, updatedCount int64) {
	// v is a single feedback value that allows us to update both alpha and beta
	var v float64 = -1
	if isSuccess {
		v = 1
	}
	newAlpha = lambda*alpha + w*(1+v)/2
	newBeta = lambda*beta + w*(1-v)/2
	return newAlpha, newBeta, totalCount + 1
}
