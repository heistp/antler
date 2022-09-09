// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"github.com/heistp/antler/node"
)

// TestRun contains the information needed to orchestrate the execution of
// Tests. A TestRun may have a Test, or nested TestRun's listed in the Serial or
// Parallel fields, which are executed sequentially or concurrently,
// respectively. TestRun's may thus be arranged in a tree to coordinate the
// serial and parallel execution of Tests.
type TestRun struct {
	// Test is the Test to run (non-nil on leaf TestRun's).
	Test *Test

	// Serial lists TestRun's to be executed sequentially.
	Serial Serial

	// Parallel lists TestRun's to be executed concurrently.
	Parallel Parallel

	doArg
}

// do executes the TestRun.
func (r *TestRun) do(ctrl *node.Control) error {
	switch {
	case len(r.Serial) > 0:
		return r.Serial.do(ctrl)
	case len(r.Parallel) > 0:
		return r.Parallel.do(ctrl)
	default:
		return r.Test.do(ctrl, r.doArg)
	}
}

// Serial is a list of TestRun's executed sequentially.
type Serial []TestRun

// do executes the TestRun's sequentially.
func (s Serial) do(ctrl *node.Control) (err error) {
	for _, u := range s {
		if err = u.do(ctrl); err != nil {
			return
		}
	}
	return
}

// Parallel is a list of TestRun's executed concurrently.
type Parallel []TestRun

// do executes the TestRun's concurrently.
func (p Parallel) do(ctrl *node.Control) (err error) {
	// TODO implement Parallel TestRuns
	return
}

// doArg contains TestRun level arguments controlling the execution of a Test.
type doArg struct {
	// Log, if true, emits LogEntry's to stdout as the Test is run.
	Log bool
}
