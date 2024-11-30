// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"encoding/gob"
	"fmt"
)

//
// message and related types
//

// A message can be sent and received by a conn.
type message interface {
	flags() flag
}

// flag is a bitmask of binary attributes on a message.
type flag uint32

const (
	flagFinal   flag = 1 << iota // final message for this direction of a conn
	flagPush                     // do not buffer or delay the message
	flagForward                  // forward directly from child to parent conn
)

//
// run and ran messages
//

// runID is the identifier for run's.
type runID uint32

// run is a message that executes a Run.
type run struct {
	ID       runID
	Run      *Run
	Feedback Feedback
	to       Node
	ran      chan ran
}

// init registers run with the gob encoder
func init() {
	gob.Register(run{})
}

// handle implements event
func (r run) handle(n *node) {
	if n.state > stateRun {
		n.rec.Logf("dropping run in state %s: %s", n.state, r)
		return
	}
	n.runc <- r
	return
}

// flags implements message
func (r run) flags() flag {
	return flagPush
}

func (r run) String() string {
	return fmt.Sprintf("run[id:%d fb:%s run:%+v]", r.ID, r.Feedback, r.Run)
}

// ran is the reply message to run.
type ran struct {
	ID       runID
	Feedback Feedback
	OK       bool
	from     Node
}

// init registers ran with the gob encoder
func init() {
	gob.Register(ran{})
}

// flags implements message
func (r ran) flags() flag {
	return flagPush
}

func (r ran) String() string {
	return fmt.Sprintf("ran[id:%d fb:%s ok:%t]", r.ID, r.Feedback, r.OK)
}

//
// setup runner
//

// setup is an internal runner used to recursively launch child nodes. It must
// run before any other Runs.
type setup struct {
	ID       runID
	Children Tree
	Exes     exes
}

// init registers setup with the gob encoder
func init() {
	gob.Register(setup{})
}

// Run implements runner
//
// Run launches and runs setup on child nodes, recursively through the node
// tree. After successful setup, the node is ready to execute Run's.
func (s setup) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	if err = repo.AddSource(s.Exes); err != nil {
		return
	}
	r := arg.rec.WithTag("launch")
	rc := make(chan ran, len(s.Children))
	for n, t := range s.Children {
		cr := r.WithTag(fmt.Sprintf("launch.%s", n))
		var c *conn
		if c, err = arg.child.Launch(n, cr.Logf); err != nil {
			return
		}
		var x exes
		if x, err = newExes(repo, t.Platforms()); err != nil {
			return
		}
		x.Remove(n.Platform)
		s := &setup{0, t, x}
		c.Run(&Run{Runners: Runners{Setup: s}}, arg.ifb, rc)
	}
	for i := 0; i < arg.child.Count(); i++ {
		select {
		case a := <-rc:
			if !a.OK {
				err = r.NewErrorf("setup on child node %s failed", a.from)
				return
			}
		case <-ctx.Done():
			err = ctx.Err()
			return
		}
	}
	return
}

// flags implements message
func (s setup) flags() flag {
	return flagPush
}

func (s setup) String() string {
	return fmt.Sprintf("setup[id:%d]", s.ID)
}

//
// cancel and canceled
//

// cancel is sent to cancel the node at any stage of its operation, including
// after normal execution. It is the final message sent from parent to child.
type cancel struct {
	Reason string
}

// init registers cancel with the gob encoder
func init() {
	gob.Register(cancel{})
}

// handle implements event
func (c cancel) handle(node *node) {
	switch node.state {
	case stateRun:
		if c.Reason != "" {
			node.setError(fmt.Errorf("parent: %s", c.Reason))
		} else {
			node.cancel = true
		}
	default:
		if c.Reason != "" {
			node.rec.Logf("ignoring cancel request for reason '%s' (state: %s)",
				c.Reason, node.state)
		}
	}
}

// flags implements message
func (c cancel) flags() flag {
	return flagPush | flagFinal
}

func (c cancel) String() string {
	return "cancel"
}

// canceled is the final message sent from child to parent.
type canceled struct{}

// init registers canceled with the gob encoder
func init() {
	gob.Register(canceled{})
}

// flags implements message
func (c canceled) flags() flag {
	return flagPush | flagFinal
}

func (c canceled) String() string {
	return "canceled"
}
