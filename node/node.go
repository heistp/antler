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
// executable runs the Do function to run a Run in an embedded Node, and
// standalone Node executables run the Serve function to run sub-trees of the
// Run in other processes, either locally or via ssh.
//
// The Node has four states: run, cancel, canceled, and done.
//
// During run, the Node handles Runs from its parent Node. Setup establishes
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

	// mutable state for run/events
	state      state
	child      *child
	cancel     bool  // true after error or cancel, starts cancellation
	runsDone   bool  // true after runs goroutine is done
	parentDone bool  // true after parent conn is done
	err        error // first error, returned from Serve()
}

// newNode returns a new node.
func newNode(nodeID string, parent transport) *node {
	ev := make(chan event)
	p := newConn(parent, Node{})
	return &node{
		ev,                             // ev
		make(chan run),                 // runc
		p,                              // parent
		newRecorder(nodeID, "node", p), // rec
		stateRun,                       // state
		newChild(ev),                   // child
		false,                          // cancel
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
func Serve(nodeID string, ctrl *Control, conn io.ReadWriteCloser) error {
	n := newNode(nodeID, newGobTransport(conn))
	if ctrl != nil {
		go ctrl.run(n.ev)
		defer ctrl.stop()
	}
	n.run()
	return n.err
}

// RootNodeID is the ID used for the root node in node.Do.
const RootNodeID = "-"

// Do runs a Run tree in an in-process "root" node, and sends data items back on
// the given channel. The item types sent can include Stream, SentMark, Sent,
// ReceivedMark, Received, FileData, LogEntry and Error.
//
// Do is used by the antler package and executable.
func Do(rn *Run, src ExeSource, ctrl *Control, data chan interface{}) {
	defer close(data)
	f := ErrorFactory{RootNodeID, "execute"}
	// run tree
	t := newTree(rn)
	x, e := newExes(src, t.Platforms())
	if e != nil {
		data <- f.NewErrore(e)
		return
	}
	// root conn
	ev := make(chan event)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for e := range ev {
			switch v := e.(type) {
			case Stream, Sent, SentMark, Received, ReceivedMark,
				FileData, LogEntry:
				data <- v
			case errorEvent:
				data <- f.NewErrore(v.err)
			case connDone:
				return
			}
		}
	}()
	tr := newChannelTransport()
	c := newConn(tr, Node{})
	c.start(ev)
	defer func() {
		c.Cancel()
		wg.Wait()
	}()
	// root node
	n := newNode(RootNodeID, tr.peer())
	if ctrl != nil {
		go ctrl.run(n.ev)
		defer ctrl.stop()
	}
	go n.run()
	// setup and run
	rc := make(chan ran, 1)
	s := &setup{0, t, x}
	c.Run(&Run{Runners: Runners{Setup: s}}, Feedback{}, rc)
	r := <-rc
	if !r.OK {
		return
	}
	c.Run(rn, r.Feedback, rc)
	data <- LogEntry{time.Now(), RootNodeID, "feedback",
		fmt.Sprintf("feedback: %s", (<-rc).Feedback)}
	return
}

// run runs the node by handling node events, and returns the first error that
// occurred.
func (n *node) run() {
	n.parent.start(n.ev)
	go n.runs()
	for e := range n.ev {
		e.handle(n)
		if d := n.advance(); d {
			break
		}
	}
}

// advance checks the current state to see if it's done. If so, it takes the
// necessary action to increment the state, repeating this until a state is
// found that's not done. advance returns true when stateDone has been reached.
func (n *node) advance() bool {
	if n.state >= stateDone {
		panic(fmt.Sprintf("can't advance past state '%s'", n.state))
	}
	for n.state.done(n) {
		n.state++
		switch n.state {
		case stateCancel:
			close(n.runc)
			n.child.Cancel()
		case stateCanceled:
			n.parent.Canceled()
		case stateDone:
			return true
		}
	}
	return false
}

// runs reads and runs Runs from the runc channel, then cancels the cancelers.
func (n *node) runs() {
	defer func() {
		n.ev <- runsDone{}
	}()
	c, d := n.canceler()
	defer func() {
		close(c)
		<-d
	}()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
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
				n.parent.Send(ran{r.ID, f, ok, r.conn})
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
		a := make([]canceler, 0, 256)
		for c := range cxl {
			a = append(a, c)
		}
		for i := len(a) - 1; i >= 0; i-- {
			c := a[i]
			r := n.rec.WithTag(typeBaseName(c))
			if e := c.Cancel(r); e != nil {
				n.ev <- errorEvent{r.NewErrore(e), false}
			}
		}
	}()
	return
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

// done returns true when the node may proceed to the next state.
func (s state) done(n *node) bool {
	switch s {
	case stateRun:
		return n.cancel
	case stateCancel:
		return n.runsDone && n.child.Count() == 0
	case stateCanceled:
		return n.parentDone
	case stateDone:
		return false
	default:
		panic(fmt.Sprintf("invalid state value %d", s))
	}
}

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

// An event can be handled by the node upon receipt on its event channel.
type event interface {
	handle(*node)
}

// errorEvent is sent when an error occurs.
type errorEvent struct {
	err error
	io  bool
}

// handle implements event
func (e errorEvent) handle(node *node) {
	node.cancel = true
	if node.err == nil {
		node.err = e.err
	}
	if e.io {
		fmt.Fprintf(os.Stderr, "%s\n", e.err)
		return
	}
	ee := node.rec.NewErrore(e.err)
	node.parent.Send(ee)
}

// runsDone is an event sent after the runs goroutine is done.
type runsDone struct {
}

// handle implements event
func (r runsDone) handle(node *node) {
	node.runsDone = true
}
