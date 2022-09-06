// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"os"
)

//
// event interface
//

// An event can be handled by the node upon receipt on its event channel.
type event interface {
	handle(*node)
}

//
// events
//

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
