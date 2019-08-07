// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"storj.io/storj/pkg/pb"
)

// Reservoir holds a certain number of segments to reflect a random sample
type Reservoir struct {
	Segments []*pb.RemoteSegment
}

// NewReservoir instantiates a Reservoir
func NewReservoir(slots int) *Reservoir {
	return &Reservoir{
		Segments: make([]*pb.RemoteSegment, slots),
	}
}
