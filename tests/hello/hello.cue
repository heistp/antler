// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

// This Antler test emits Hello World in the root node.

package hello

// Test contains a single Test that emits Hello World.
Test: [{
	// System is a system command.
	System: {
		Command: "bash -c"
		Arg: [ "echo Hello World!"]
	}
	// disable saving of gob data
	DataFile: ""
	// remove default reporters to skip writing any files
	AfterDefault: []
}]
