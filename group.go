// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

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

	// During is a pipeline of Reports run while the Test runs.
	During Report

	// DuringDefault is the default pipeline of Reports run while the Test runs.
	DuringDefault Report

	// After is a pipeline of Reports run after the Test completes.
	After Report

	// After is the default pipeline of Reports run after the Test completes.
	AfterDefault Report
}
