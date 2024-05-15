// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import "context"

// Group is used to form a hierarchy of Tests. Each Group is a node in the
// hierarchy containing a list of Tests with the same ID keys, and a list of
// sub-Groups.
type Group struct {
	// Name is the name of the Group, and is used as the name of the directory
	// containing the results for the Group.
	Name string

	// Test lists the Tests in the Group, and may be empty for Groups that only
	// contain other Groups.
	Test []Test

	// Group lists any sub-Groups of the Group.
	Group []Group

	// During is a pipeline of Reports run while the Tests run.
	During Report

	// DuringDefault is the default pipeline of Reports run while the Tests run.
	DuringDefault Report

	// After is a pipeline of Reports run after the Tests complete.
	After Report

	// After is the default pipeline of Reports run after the Tests complete.
	AfterDefault Report
}

// VisitTests calls the given visitor func for each Test in the Group
// hierarchy. The visitor may return false to stop visiting, in which case
// VisitTests will also return false.
func (g *Group) VisitTests(visitor func(*Test) bool) bool {
	for _, t := range g.Test {
		if !visitor(&t) {
			return false
		}
	}
	for _, g := range g.Group {
		if !g.VisitTests(visitor) {
			return false
		}
	}
	return true
}

// do runs a doer on the Tests, and recursively on the sub-Groups.
func (g *Group) do(ctx context.Context, d doer2) (err error) {
	for _, t := range g.Test {
		if err = d.do(ctx, &t); err != nil {
			return
		}
	}
	for _, g := range g.Group {
		if err = g.do(ctx, d); err != nil {
			return
		}
	}
	return
}

// A doer2 takes action on a Test, visited in a Group hierarchy.
type doer2 interface {
	do(context.Context, *Test) error
}
