// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"cuelang.org/go/cue/load"
)

// TestRun contains the information needed to orchestrate the execution of Tests
// and Reports. A TestRun may have a Test, or nested TestRun's listed in the
// Serial or Parallel fields, which are executed sequentially or concurrently,
// respectively. TestRun's may thus be arranged in a tree to coordinate the
// serial and parallel execution of Tests.
type TestRun struct {
	// Test is the Test to run (non-nil on leaf TestRun's).
	Test *Test

	// Report lists Reports to be run on this TestRun and any below it in the
	// TestRun tree.
	Report reports

	// Serial lists TestRun's to be executed sequentially.
	Serial Serial

	// Parallel lists TestRun's to be executed concurrently.
	Parallel Parallel
}

// do runs a doer, observing the Serial and Parallel structure of the TestRun.
func (t *TestRun) do(dr doer, rst reporterStack) (err error) {
	rst.push(t.Report.reporters())
	defer func() {
		if e := rst.pop(); e != nil && err == nil {
			err = e
		}
	}()
	switch {
	case len(t.Serial) > 0:
		err = t.Serial.do(dr, rst)
	case len(t.Parallel) > 0:
		err = t.Parallel.do(dr, rst)
	default:
		err = t.Test.do(dr, rst)
	}
	return
}

// do loads the configuration and calls do on the top-level TestRun.
func do(dr doer) (err error) {
	var cfg *Config
	if cfg, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	err = cfg.Run.do(dr, reporterStack{})
	return
}

// A doer takes action on a Test, visited in a TestRun tree.
type doer interface {
	do(*Test, reporterStack) error
}

// Serial is a list of TestRun's executed sequentially.
type Serial []TestRun

// do executes the TestRun's sequentially.
func (s Serial) do(dr doer, rst reporterStack) (err error) {
	for _, r := range s {
		if err = r.do(dr, rst); err != nil {
			return
		}
	}
	return
}

// Parallel is a list of TestRun's executed concurrently.
type Parallel []TestRun

// do executes the TestRun's concurrently.
func (p Parallel) do(dr doer, rst reporterStack) (err error) {
	c := make(chan error)
	for _, r := range p {
		r := r
		go func() {
			var e error
			defer func() {
				c <- e
			}()
			e = r.do(dr, rst)
		}()
	}
	for i := 0; i < len(p); i++ {
		if e := <-c; e != nil && err == nil {
			err = e
		}
	}
	return
}
