// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"fmt"
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

	// Child is a Run to be executed on a child Node
	Child *Child

	// Runners is a union of the available runner implementations.
	//
	// NOTE: In the future, this may be an interface field, if CUE can be made
	// to choose a concrete type without using a field for each runner.
	Runners
}

// run runs the Run.
func (r *Run) run(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler, ev chan event) (ofb Feedback, ok bool) {
	switch {
	case len(r.Serial) > 0:
		ofb, ok = r.Serial.do(ctx, chl, ifb, rec, cxl, ev)
	case len(r.Parallel) > 0:
		ofb, ok = r.Parallel.do(ctx, chl, ifb, rec, cxl, ev)
	case r.Child != nil:
		ofb, ok = r.Child.do(ctx, chl, ifb, rec, cxl, ev)
	default:
		ofb, ok = r.Runners.do(ctx, chl, ifb, rec, cxl, ev)
	}
	return
}

// Serial is a list of Runs executed sequentially.
type Serial []Run

// do executes the Serial Runs sequentially.
func (s Serial) do(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler, ev chan event) (ofb Feedback, ok bool) {
	ofb = Feedback{}
	for _, r := range s {
		var f Feedback
		f, ok = r.run(ctx, chl, ifb, rec, cxl, ev)
		if e := ofb.merge(f); e != nil {
			ok = false
			rr := rec.WithTag(typeBaseName(r))
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

// parallelRan is the result returned by Parallel.do's internal goroutine.
type parallelRan struct {
	run *Run
	ofb Feedback
	ok  bool
}

// do executes the Parallel Runs concurrently.
func (p Parallel) do(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler, ev chan event) (ofb Feedback, ok bool) {
	ofb = Feedback{}
	c := make(chan parallelRan)
	for _, r := range p {
		r := r
		go func() {
			var a parallelRan
			defer func() {
				c <- a
			}()
			a.run = &r
			a.ofb, a.ok = r.run(ctx, chl, ifb, rec, cxl, ev)
		}()
	}
	ok = true
	for i := 0; i < len(p); i++ {
		a := <-c
		if e := ofb.merge(a.ofb); e != nil {
			ok = false
			rr := rec.WithTag(typeBaseName(a.run))
			ev <- errorEvent{rr.NewErrore(e), false}
		}
		if !a.ok {
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
func (r *Child) do(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler, ev chan event) (ofb Feedback, ok bool) {
	c := chl.Get(r.Node)
	rc := make(chan ran, 1)
	c.Run(&r.Run, ifb, rc)
	a := <-rc
	ofb = a.Feedback
	ok = a.OK
	return
}

// Runners is a union of the available runner implementations. Only one of the
// runners may be non-nil.
type Runners struct {
	Sleep  *Sleep
	System *System
	Setup  *setup
}

// Runner returns the only non-nil runner implementation.
func (r *Runners) runner() runner {
	switch {
	case r.Sleep != nil:
		return r.Sleep
	case r.System != nil:
		return r.System
	case r.Setup != nil:
		return r.Setup
	}
	return nil
}

// do executes the runner.
func (r *Runners) do(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler, ev chan event) (ofb Feedback, ok bool) {
	var u runner
	if u = r.runner(); u == nil {
		e := rec.NewErrorf("Run has no runner set")
		ev <- errorEvent{e, false}
		return
	}
	rr := rec.WithTag(typeBaseName(u))
	var err error
	ofb, err = u.Run(ctx, chl, ifb, rr, cxl)
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

// runner is the interface that wraps the run method. runners are passed to a
// node for execution, and are used for all node calls, from child connection
// setup, to test environment setup, to test clients and servers.
//
// When Context is canceled, runners should return as soon as possible, using
// Context.Err() as the returned error if the cancellation materially affects
// the results.
//
// The child argument caches child conns.
//
// The Feedback argument contains incoming Feedback from prior runners, while
// the returned Feedback contains outgoing Feedback for subsequent runners.
type runner interface {
	Run(context.Context, *child, Feedback, *recorder, chan canceler) (
		Feedback, error)
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
