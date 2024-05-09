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

	// ResultPrefix is the base path for any output files. It may use Go
	// template syntax (https://pkg.go.dev/text/template), with the Test ID
	// passed to the template as its data. ResultPrefix must be unique for each
	// Test in the Group, and may be empty for a single Test.
	ResultPrefix string
}
