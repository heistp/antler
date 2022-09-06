// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// Control is used to send node control signals.
type Control struct {
	cancelC chan string
	done    chan struct{}
}

// NewControl returns a new Control.
func NewControl() *Control {
	c := &Control{
		make(chan string),
		make(chan struct{}),
	}
	return c
}

// Cancel sends a cancellation request to the node.
func (c *Control) Cancel(reason string) {
	select {
	case c.cancelC <- reason:
	case <-c.done:
	}
}

// run is the Control goroutine's main entry point.
func (c *Control) run(ev chan<- event) {
	var e chan<- event
	var h event
	d := false
	for !d {
		select {
		case r := <-c.cancelC:
			h = cancel{r}
			e = ev
		case e <- h:
			h = nil
			e = nil
		case <-c.done:
			d = true
		}
	}
}

// stop stops the goroutine and releases any resources.
func (c *Control) stop() {
	close(c.done)
}
