// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"

	"github.com/heistp/antler/node/metric"
)

// A scheduler can trigger a send, write or flow of a length given by the Len
// field in tick. If the Reply field in tick is set to a non-nil channel, the
// scheduler will receive a tick in the in channel when any reply related to the
// tick is received.
//
// When the scheduler is complete, it must send a tick to the channel with the
// Done field set to true.
type scheduler interface {
	schedule(context.Context, chan tick)
}

// A tick is the unit of work sent and received by a scheduler.
type tick struct {
	// Len is the length of the send, write or flow to trigger.
	Len int

	// Reply, if not nil, indicates that a reply to the tick is requested on
	// this channel, which may be used to trigger further ticks. Depending on
	// how the ticks are used, replies may or may not be received, so schedulers
	// should be prepared for this.
	Reply chan tick

	// Done indicates that the scheduler is done generating ticks. When Done is
	// true, no further action is taken on the tick.
	Done bool
}

// Schedulers is the union of available scheduler implementations.
type Schedulers struct {
	Isochronous *Isochronous
}

// scheduler returns the only non-nil scheduler implementation.
func (s *Schedulers) scheduler() scheduler {
	switch {
	case s.Isochronous != nil:
		return s.Isochronous
	default:
		panic("no scheduler set in schedulers union")
	}
}

// Isochronous sends ticks on a periodic schedule with fixed interval.
type Isochronous struct {
	// Interval is the fixed time between ticks.
	Interval metric.Duration
}

// schedule implements scheduler
func (*Isochronous) schedule(ctx context.Context, out chan tick) {
	// TODO implement Isochronous.schedule
}
