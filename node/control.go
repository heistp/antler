// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// Control is used to send control signals to nodes.
type Control struct {
	attachC chan chan<- event
	ev      []chan<- event
	cancel  chan string
}

// NewControl returns a new Control.
func NewControl() Control {
	c := Control{
		make(chan chan<- event),
		nil,
		make(chan string),
	}
	go c.run()
	return c
}

// Cancel sends a cancellation request to all attached nodes.
func (c Control) Cancel(reason string) {
	c.cancel <- reason
}

// Stop releases any resources. Cancel must not be called after Stop.
func (c Control) Stop() {
	close(c.cancel)
}

// attach adds a node's event channel for notification of cancellations. The
// channel is only notified of up to one cancel event.
func (c Control) attach(ev chan<- event) {
	c.attachC <- ev
}

// run is the Control goroutine's main entry point, called by the node.
func (c Control) run() {
	for {
		select {
		case ev := <-c.attachC:
			c.ev = append(c.ev, ev)
		case r, ok := <-c.cancel:
			if !ok {
				return
			}
			for _, ev := range c.ev {
				select {
				case ev <- cancel{r}:
				default:
				}
			}
			c.ev = nil
		}
	}
}
