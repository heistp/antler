// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import "github.com/heistp/antler/node/metric"

// A scheduler can trigger a send, write or flow of a length given by the Len
// field in tick. If the Reply field in tick is true, the scheduler will receive
// a tick in the in channel when any reply related to a tick is received.
//
// The scheduler must read the in channel fully, and when closed, the scheduler
// must complete, and send a tick to the out channel with the Done field set to
// true.
type scheduler interface {
	schedule(in, out chan tick)
}

// A tick is the unit of work sent and received by a scheduler.
type tick struct {
	// Len is the length of the send, write or flow to trigger.
	Len int

	// Reply, if true, indicates that a reply to the send is requested and
	// expected on the scheduler's in channel, which may be used to trigger
	// further ticks. Depending on how the ticks are used, replies may or may
	// not be received, so schedulers should be prepared for this.
	Reply bool

	// Done indicates that the scheduler is done generating ticks.
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
func (*Isochronous) schedule(in, out chan tick) {
	// TODO implement Isochronous.schedule
}
