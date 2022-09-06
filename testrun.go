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
	Test     *Test    // Test to run (non-nil on leaf TestRun's)
	Serial   Serial   // lists TestRun's to be executed sequentially
	Parallel Parallel // lists TestRun's to be executed concurrently
}

// Do executes the TestRun.
func (r *TestRun) Do(ctrl *node.Control) error {
	switch {
	case len(r.Serial) > 0:
		return r.Serial.Do(ctrl)
	case len(r.Parallel) > 0:
		return r.Parallel.Do(ctrl)
	default:
		return r.Test.Do(ctrl)
	}
}

// Serial is a list of TestRun's executed sequentially.
type Serial []TestRun

// Do executes the TestRun's sequentially.
func (s Serial) Do(ctrl *node.Control) (err error) {
	for _, u := range s {
		if err = u.Do(ctrl); err != nil {
			return
		}
	}
	return
}

// Parallel is a list of TestRun's executed concurrently.
type Parallel []TestRun

// Do executes the TestRun's concurrently.
func (p Parallel) Do(ctrl *node.Control) (err error) {
	// TODO implement Parallel TestRuns
	return
}
