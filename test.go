// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"os"
	"path"
	"strings"

	"github.com/heistp/antler/node"
)

// Test is an Antler test.
type Test struct {
	// OutPath is the base path for test output files, relative to the output
	// directory. Paths ending in '/' are a directory, and '/' is appended
	// automatically if the path is a directory. The default is "./".
	OutPath string

	// Run is the top-level Run instance.
	node.Run
}

// ID represents a compound Test identifier consisting of key/value pairs.
type ID map[string]string

// do calls the given doer on the Test.
func (t *Test) do(dr doer, rst reporterStack) (err error) {
	return dr.do(t, rst)
}

// outPath returns the path to an output file with the given suffix. OutPath is
// first normalized, appending "/" to the path if it refers to a directory.
func (t *Test) outPath(suffix string) string {
	p := t.OutPath
	var d bool
	if strings.HasSuffix(p, "/") {
		d = true
	} else if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		d = true
		p += "/"
	}
	if d {
		return path.Join(p, suffix)
	}
	return p + "_" + suffix
}
