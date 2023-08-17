// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

// TestRun contains the information needed to orchestrate the execution of Tests
// and Reports. A TestRun may have a Test, or nested TestRun's listed in the
// Serial or Parallel fields, which are executed sequentially or concurrently,
// respectively. TestRun's may thus be arranged in a hierarchy to coordinate the
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

// VisitTests calls the given visitor func for each Test in the TestRun
// hierarchy. The visitor may return false to stop visiting, in which case
// VisitTests will also return false.
func (t *TestRun) VisitTests(visitor func(*Test) bool) bool {
	var rr []TestRun
	switch {
	case len(t.Serial) > 0:
		rr = t.Serial
	case len(t.Parallel) > 0:
		rr = t.Parallel
	default:
		return visitor(t.Test)
	}
	for _, r := range rr {
		if !r.VisitTests(visitor) {
			return false
		}
	}
	return true
}

// do runs a doer, observing the Serial and Parallel structure of the TestRun.
func (t *TestRun) do(d doer, rst reporterStack) (err error) {
	rst.push(t.Report.reporters())
	defer rst.pop()
	switch {
	case len(t.Serial) > 0:
		err = t.Serial.do(d, rst)
	case len(t.Parallel) > 0:
		err = t.Parallel.do(d, rst)
	default:
		err = t.Test.do(d, rst)
	}
	return
}

// A doer takes action on a Test, visited in a TestRun tree.
type doer interface {
	do(*Test, reporterStack) error
}

// Serial is a list of TestRun's executed sequentially.
type Serial []TestRun

// do executes the TestRun's sequentially.
func (s Serial) do(d doer, rst reporterStack) (err error) {
	for _, r := range s {
		if err = r.do(d, rst); err != nil {
			return
		}
	}
	return
}

// Parallel is a list of TestRun's executed concurrently.
type Parallel []TestRun

// do executes the TestRun's concurrently.
func (p Parallel) do(d doer, rst reporterStack) (err error) {
	c := make(chan error)
	for _, r := range p {
		r := r
		go func() {
			var e error
			defer func() {
				c <- e
			}()
			e = r.do(d, rst)
		}()
	}
	for i := 0; i < len(p); i++ {
		if e := <-c; e != nil && err == nil {
			err = e
		}
	}
	return
}
