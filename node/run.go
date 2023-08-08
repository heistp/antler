// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/heistp/antler/node/metric"
)

//
// Run and related types
//

// Run represents the information needed to coordinate the execution of runners.
// Using the Serial, Parallel and Child fields, Runs may be arranged in a tree
// for sequential, concurrent and child node execution.
//
// Run must be created with valid constraints, i.e. each Run must have exactly
// one of Serial, Parallel, Child or a Runners field set. Run is not safe for
// concurrent use, though Parallel Runs execute safely, concurrently.
type Run struct {
	// Serial lists Runs to be executed sequentially
	Serial Serial

	// Parallel lists Runs to be executed concurrently
	Parallel Parallel

	// Schedule lists Runs to be executed on a schedule.
	Schedule *Schedule

	// Child is a Run to be executed on a child Node
	Child *Child

	// Runners is a union of the available runner implementations.
	//
	// NOTE: In the future, this may be an interface field, if CUE can be made
	// to choose a concrete type without using a field for each runner.
	Runners
}

// run runs the Run.
func (r *Run) run(ctx context.Context, arg runArg, ev chan event) (
	ofb Feedback, ok bool) {
	switch {
	case len(r.Serial) > 0:
		ofb, ok = r.Serial.do(ctx, arg, ev)
	case len(r.Parallel) > 0:
		ofb, ok = r.Parallel.do(ctx, arg, ev)
	case r.Schedule != nil:
		ofb, ok = r.Schedule.do(ctx, arg, ev)
	case r.Child != nil:
		ofb, ok = r.Child.do(ctx, arg, ev)
	default:
		ofb, ok = r.Runners.do(ctx, arg, ev)
	}
	return
}

// Serial is a list of Runs executed sequentially.
type Serial []Run

// do executes the Serial Runs sequentially.
func (s Serial) do(ctx context.Context, arg runArg, ev chan event) (
	ofb Feedback, ok bool) {
	ofb = Feedback{}
	for _, r := range s {
		var f Feedback
		f, ok = r.run(ctx, arg, ev)
		if e := ofb.merge(f); e != nil {
			ok = false
			rr := arg.rec.WithTag(typeBaseName(r))
			ev <- errorEvent{rr.NewErrore(e), false}
		}
		if !ok {
			return
		}
	}
	return
}

// Parallel is a list of Runs executed concurrently.
type Parallel []Run

// do executes the Parallel Runs concurrently.
func (p Parallel) do(ctx context.Context, arg runArg, ev chan event) (
	ofb Feedback, ok bool) {
	ofb = Feedback{}
	c := make(chan runDone)
	for _, r := range p {
		r := r
		go func() {
			var d runDone
			defer func() {
				c <- d
			}()
			d.run = &r
			d.ofb, d.ok = r.run(ctx, arg, ev)
		}()
	}
	ok = true
	for i := 0; i < len(p); i++ {
		d := <-c
		if e := ofb.merge(d.ofb); e != nil {
			ok = false
			rr := arg.rec.WithTag(typeBaseName(d.run))
			ev <- errorEvent{rr.NewErrore(e), false}
		}
		if !d.ok {
			ok = false
		}
	}
	return
}

// Child is a Run to execute on a child Node.
type Child struct {
	// Run is the Run to execute on Node.
	Run

	// Node is the node to execute Run on. It must be a valid, nonzero value.
	Node Node
}

// do executes Child's Run on a child node.
func (r *Child) do(ctx context.Context, arg runArg, ev chan event) (
	ofb Feedback, ok bool) {
	c := arg.child.Get(r.Node)
	rc := make(chan ran, 1)
	c.Run(&r.Run, arg.ifb, rc)
	a := <-rc
	ofb = a.Feedback
	ok = a.OK
	return
}

// Schedule lists Runs to be executed with wait times between each Run.
type Schedule struct {
	// Wait lists the wait Durations to use. If Random is false, the chosen
	// Durations cycle repeatedly through Wait.
	Wait []metric.Duration

	// WaitFirst, if true, indicates to wait before the first Run as well.
	WaitFirst bool

	// Random, if true, indicates to select wait times from Wait randomly.
	// Otherwise, wait times are taken from Wait sequentially.
	Random bool

	// Sequential, if true, indicates to run the Runs in serial.
	Sequential bool

	// Run lists the Runs.
	Run []Run

	// waitIndex is the current index in Wait.
	waitIndex int

	// rand provides random wait times when Random is true.
	rand *rand.Rand
}

// do executes Schedule's Runs on a schedule.
func (s *Schedule) do(ctx context.Context, arg runArg, ev chan event) (
	ofb Feedback, ok bool) {
	ofb = Feedback{}
	ok = true
	var g, i int
	r := make(chan runDone)
	dc := ctx.Done()
	w := time.After(s.firstWait())
	for (i < len(s.Run) && dc != nil && ok) || g > 0 {
		select {
		case <-w:
			if dc == nil || !ok {
				break
			}
			g++
			go func(run *Run) {
				var d runDone
				defer func() {
					r <- d
				}()
				d.run = run
				d.ofb, d.ok = run.run(ctx, arg, ev)
			}(&s.Run[i])
			if i++; i < len(s.Run) && !s.Sequential {
				w = time.After(s.nextWait())
			}
		case d := <-r:
			g--
			if e := ofb.merge(d.ofb); e != nil {
				ok = false
				rr := arg.rec.WithTag(typeBaseName(d.run))
				ev <- errorEvent{rr.NewErrore(e), false}
				break
			}
			if !d.ok {
				ok = false
			}
			if s.Sequential && dc != nil && ok && i < len(s.Run) {
				w = time.After(s.nextWait())
			}
		case <-dc:
			dc = nil
		}
	}
	return
}

// firstWait returns the first wait time.
func (s *Schedule) firstWait() time.Duration {
	if !s.WaitFirst {
		return 0
	}
	return s.nextWait()
}

// nextWait returns the next wait time.
func (s *Schedule) nextWait() (wait time.Duration) {
	if len(s.Wait) == 0 {
		return
	}
	if s.Random {
		if s.rand == nil {
			s.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		wait = time.Duration(s.Wait[s.rand.Intn(len(s.Wait))])
		return
	}
	wait = time.Duration(s.Wait[s.waitIndex])
	if s.waitIndex++; s.waitIndex >= len(s.Wait) {
		s.waitIndex = 0
	}
	return
}

// runDone is the result returned by Run's internal goroutines.
type runDone struct {
	run *Run
	ofb Feedback
	ok  bool
}

// Runners is a union of the available runner implementations. Only one of the
// runners may be non-nil.
type Runners struct {
	ResultStream *ResultStream
	Setup        *setup
	Sleep        *Sleep
	System       *System
	StreamClient *StreamClient
	StreamServer *StreamServer
	PacketServer *PacketServer
	PacketClient *PacketClient
}

// runner returns the only non-nil runner implementation.
func (r *Runners) runner() runner {
	switch {
	case r.ResultStream != nil:
		return r.ResultStream
	case r.Setup != nil:
		return r.Setup
	case r.Sleep != nil:
		return r.Sleep
	case r.System != nil:
		return r.System
	case r.StreamClient != nil:
		return r.StreamClient
	case r.StreamServer != nil:
		return r.StreamServer
	case r.PacketClient != nil:
		return r.PacketClient
	case r.PacketServer != nil:
		return r.PacketServer
	}
	return nil
}

// do executes the runner.
func (r *Runners) do(ctx context.Context, arg runArg, ev chan event) (
	ofb Feedback, ok bool) {
	var u runner
	if u = r.runner(); u == nil {
		e := arg.rec.NewErrorf("Run has no runner set")
		ev <- errorEvent{e, false}
		return
	}
	rr := arg.rec.WithTag(typeBaseName(u))
	var err error
	ofb, err = u.Run(ctx, arg)
	if ofb == nil {
		ofb = Feedback{}
	}
	if err != nil {
		ev <- errorEvent{rr.NewErrore(err), false}
		return
	}
	ok = true
	return
}

//
// runner interface and related types
//

// runner is the interface that wraps the Run method. runners are passed to a
// node for execution, and are used for all node calls, from child connection
// setup, to test environment setup, to test clients and servers.
//
// When Context is canceled, runners should return as soon as possible, using
// Context.Err() as the returned error if the cancellation materially affects
// the results.
type runner interface {
	Run(context.Context, runArg) (Feedback, error)
}

// runArg contains the arguments supplied to a runner.
type runArg struct {
	child    *child        // caches child conns
	ifb      Feedback      // incoming Feedback from prior runners
	sockdiag *sockdiag     // access to socket information on Linux
	rec      *recorder     // recorder for logging, data and errors
	cxl      chan canceler // canceler stack
}

// canceler is the interface that wraps the Cancel method. If a runner
// implements canceler and its run method returns successfully, the Cancel
// method will be called before the node exits to perform cleanup operations.
// canceler's are called sequentially, in reverse order from the order in which
// their corresponding runners were called.
type canceler interface {
	Cancel(*recorder) error
}

// Feedback contains key/value pairs, which are returned by runners for use by
// subsequent runners, and are stored in the result Data. Values must be
// supported by gob.
type Feedback map[string]interface{}

// merge merges the given Feedback f2 into this Feedback. An error is returned
// if any of f2's keys already exist in f.
func (f Feedback) merge(f2 Feedback) (err error) {
	for k2, v2 := range f {
		if v, ok := f[k2]; ok {
			err = fmt.Errorf("feedback conflict merging %s=%+v into %s=%+v",
				k2, v2, k2, v)
			return
		}
		f[k2] = v2
	}
	return
}
