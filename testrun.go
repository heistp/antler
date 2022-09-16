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

	// Report lists Reports to be run on this TestRun and any below it in the
	// TestRun tree.
	Report reports
}

// do executes the TestRun.
func (t *TestRun) do(ctrl *node.Control, rst reporterStack) (err error) {
	rst.push(t.Report.reporters())
	defer func() {
		if e := rst.pop(); e != nil && err == nil {
			err = e
		}
	}()
	switch {
	case len(t.Serial) > 0:
		err = t.Serial.do(ctrl, rst)
	case len(t.Parallel) > 0:
		err = t.Parallel.do(ctrl, rst)
	default:
		err = t.Test.do(ctrl, rst)
	}
	return
}

// Serial is a list of TestRun's executed sequentially.
type Serial []TestRun

// do executes the TestRun's sequentially.
func (s Serial) do(ctrl *node.Control, rst reporterStack) (err error) {
	for _, u := range s {
		if err = u.do(ctrl, rst); err != nil {
			return
		}
	}
	return
}

// Parallel is a list of TestRun's executed concurrently.
type Parallel []TestRun

// do executes the TestRun's concurrently.
func (p Parallel) do(ctrl *node.Control, rst reporterStack) (err error) {
	// TODO implement Parallel TestRuns
	return
}
