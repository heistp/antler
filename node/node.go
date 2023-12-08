// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package node contains the node implementation, Run, and related types.

package node

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// node is a combined client and server that runs Run trees. The main antler
// executable runs the Do function to run a Run in an embedded Node, and the
// standalone node executable runs the Serve function to run sub-trees of the
// Run in other processes, either locally or via ssh.
//
// The Node has four states: run, cancel, canceled, and done.
//
// During run, the node handles Runs from its parent node. Setup establishes
// connections to zero or more child nodes, and run executes a Run tree,
// sometimes using child nodes in the process.
//
// Upon error or cancellation, cancel begins. Any local Runs are canceled,
// and their cancelers are run. Any child connections are canceled, then closed.
//
// After the local Runs are canceled and the children are closed or time out,
// canceled begins, whereupon 'Canceled' is sent to the parent, flushing any
// buffered data.
//
// Once the parent conn is done, the node is done.
type node struct {
	// immutable from construction
	ev     chan event
	runc   chan run
	parent *conn
	rec    *recorder
	child  *child

	// mutable state for run/events
	state       state
	cancel      bool  // true after normal cancel request
	contextDone bool  // true after context is done
	runsDone    bool  // true after runs goroutine is done
	parentDone  bool  // true after parent conn is done
	err         error // first error, returned from Serve()
}

// newNode returns a new node.
func newNode(nodeID ID, parent transport) *node {
	ev := make(chan event)
	p := newConn(parent, ParentNode)
	return &node{
		ev,                             // ev
		make(chan run),                 // runc
		p,                              // parent
		newRecorder(nodeID, "node", p), // rec
		newChild(ev),                   // child
		stateRun,                       // state
		false,                          // cancel
		false,                          // contextDone
		false,                          // runsDone
		false,                          // parentDone
		nil,                            // err
	}
}

// Serve runs a node whose parent is connected using the given conn. This is
// used by the standalone node executable.
//
// An error is returned if there was a failure when serving the connection, or
// the node was explicitly canceled. Serve closes the conn when complete.
func Serve(ctx context.Context, nodeID ID, conn io.ReadWriteCloser) error {
	n := newNode(nodeID, newGobTransport(conn))
	n.run(ctx)
	return n.err
}

// RootNodeID is the ID used for the root node in node.Do.
const RootNodeID = "antler"

// Do runs a Run tree in an in-process "root" node, and sends data items back on
// the given data channel. The item types that may be sent include StreamInfo,
// StreamIO, PacketInfo, PacketIO, FileData, SysInfoData, LogEntry and Error.
//
// Do is used by the antler package and executable.
func Do(ctx context.Context, rn *Run, src ExeSource, data chan any) {
	defer close(data)
	f := ErrorFactory{RootNodeID, "do"}
	var err error
	defer func() {
		if err != nil {
			data <- f.NewErrore(err)
		}
	}()
	// run tree
	t := NewTree(rn)
	// executables
	var x exes
	if x, err = newExes(src, t.Platforms()); err != nil {
		return
	}
	// root conn
	ev := make(chan event)
	var w sync.WaitGroup
	w.Add(1)
	go func() {
		defer w.Done()
		for e := range ev {
			switch v := e.(type) {
			case connDone:
				return
			case errorEvent:
				data <- f.NewErrore(v.err)
			default:
				data <- v
			}
		}
	}()
	tr := newChannelTransport()
	c := newConn(tr, ParentNode)
	c.start(ev)
	defer func() {
		c.Cancel()
		w.Wait()
	}()
	// root node
	n := newNode(RootNodeID, tr.peer())
	go n.run(ctx)
	// setup and run
	rc := make(chan ran, 1)
	c.Run(&Run{Runners: Runners{Setup: &setup{0, t, x}}}, Feedback{}, rc)
	r := <-rc
	if !r.OK {
		return
	}
	c.Run(rn, r.Feedback, rc)
	if k := (<-rc).Feedback; len(k) > 0 {
		data <- LogEntry{time.Now(), RootNodeID, "feedback",
			fmt.Sprintf("feedback: %s", k)}
	}
	return
}

// run runs the node by handling node events and advancing the state.
func (n *node) run(ctx context.Context) {
	ctx, x := context.WithCancelCause(ctx)
	defer x(nil)
	n.parent.start(n.ev)
	go n.waitContext(ctx)
	go n.handleRuns(ctx)
	for e := range n.ev {
		e.handle(n)
		if !n.advance(x) {
			break
		}
	}
}

// advance checks the current state to see if it's done. If so, it enters the
// next state, repeating this until a state is found that's not done. False is
// returned when stateDone is reached.
func (n *node) advance(cxl context.CancelCauseFunc) bool {
	for {
		// check if current state is done
		var d bool
		switch n.state {
		case stateRun:
			d = n.err != nil || n.cancel || n.contextDone
		case stateCancel:
			d = n.runsDone && n.child.Count() == 0 && n.contextDone
		case stateCanceled:
			d = n.parentDone
		case stateDone:
			return false
		default:
			panic(fmt.Sprintf("invalid check state: %d", n.state))
		}

		// if not done, return true
		if !d {
			return true
		}

		// enter next state
		n.state++
		switch n.state {
		case stateCancel:
			cxl(n.err)
			close(n.runc)
			n.child.Cancel()
		case stateCanceled:
			n.parent.Canceled()
		case stateDone:
			return false
		default:
			panic(fmt.Sprintf("invalid enter state: %d", n.state))
		}
	}
}

// waitContext sends a contextDone event when ctx.Done() is closed.
func (n *node) waitContext(ctx context.Context) {
	<-ctx.Done()
	n.ev <- contextDone{context.Cause(ctx)}
}

// handleRuns receives and handles Runs from the runc channel until it's closed,
// then cancels the cancelers.
func (n *node) handleRuns(ctx context.Context) {
	defer func() {
		n.ev <- runsDone{}
	}()
	c, d := n.canceler()
	defer func() {
		close(c)
		<-d
	}()
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
	}()
	for r := range n.runc {
		r := r
		wg.Add(1)
		go func() {
			var f Feedback
			var ok bool
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					e := fmt.Errorf("run panic: %s\n%s", p,
						string(debug.Stack()))
					n.ev <- errorEvent{e, false}
				}
				if f == nil {
					f = Feedback{}
				}
				n.parent.Send(ran{r.ID, f, ok, r.to})
			}()
			f, ok = r.Run.run(ctx, runArg{n.child, r.Feedback, n.rec, c}, n.ev)
		}()
	}
}

// canceler confines a goroutine to read cancelers from cxl and push them onto a
// stack. When cxl is closed, it pops and runs the cancelers from the stack,
// then closes done.
func (n *node) canceler() (cxl chan canceler, done chan struct{}) {
	cxl = make(chan canceler)
	done = make(chan struct{})
	go func() {
		defer close(done)
		var a []canceler
		for c := range cxl {
			a = append(a, c)
		}
		for i := len(a) - 1; i >= 0; i-- {
			c := a[i]
			if e := c.Cancel(); e != nil {
				n.ev <- errorEvent{n.rec.NewErrore(e), false}
			}
		}
	}()
	return
}

// setError sets the node.err field, on the first error only.
func (n *node) setError(err error) {
	if n.err != nil {
		return
	}
	n.err = err
}

//
// node states
//

// state represents the possible node states.
type state uint

const (
	stateRun state = iota
	stateCancel
	stateCanceled
	stateDone
)

func (s state) String() string {
	switch s {
	case stateRun:
		return "Run"
	case stateCancel:
		return "Cancel"
	case stateCanceled:
		return "Canceled"
	case stateDone:
		return "Done"
	default:
		panic(fmt.Sprintf("invalid state value %d", s))
	}
}

//
// event interface and related types
//

// An event can be handled by the node upon receipt on its event channel. Event
// handlers need not be safe for concurrent use, but must not send another event
// to the event channel, or a deadlock will occur.
type event interface {
	handle(*node)
}

// contextDone is an event sent when the node's Context is done.
type contextDone struct {
	err error
}

// handle implements event
func (d contextDone) handle(node *node) {
	node.contextDone = true
	if d.err != context.Canceled {
		node.setError(d.err)
		node.rec.Logf("context canceled for reason: %s", d.err)
	}
}

// errorEvent is an event sent when an error occurs.
type errorEvent struct {
	err error
	io  bool
}

// handle implements event
func (e errorEvent) handle(node *node) {
	node.setError(e.err)
	if e.io {
		fmt.Fprintf(os.Stderr, "%s: %s\n", node.rec.nodeID, e.err)
		return
	}
	ee := node.rec.NewErrore(e.err)
	node.parent.Send(ee)
}

// runsDone is an event sent after the runs goroutine is done.
type runsDone struct {
}

// handle implements event
func (runsDone) handle(node *node) {
	node.runsDone = true
}
