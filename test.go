// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"os"

	"github.com/heistp/antler/node"
)

// Test is an Antler test.
type Test struct {
	// Props maps property names to values, which together are used to uniquely
	// identify a Test within a CUE package. These properties may be used to
	// generate output filenames and/or report lists/tables that identify the
	// key Test properties, e.g. bandwidth, rtt, etc.
	Props map[string]interface{}

	// Run is the top-level Run instance.
	node.Run
}

// Do runs the Test and saves the Result.
func (t *Test) Do(ctrl *node.Control) (err error) {
	c := make(chan interface{}, 64)
	d := make(chan struct{})
	r := newResult(*t)
	go r.gather(c, d)
	node.Do(&t.Run, &exeSource{}, ctrl, c)
	<-d
	r.DumpText(os.Stdout)
	return
}
