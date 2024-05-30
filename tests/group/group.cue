// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

// This Antler test exercises basic Group functionality.

package group

// Test defines a single Test that emits Hello World.
Group: {
	// Test lists the Tests in the default Group.
	Test: [{
		// System is a system command.
		System: {
			Command: "bash -c"
			Arg: [ "echo Test in default group"]
		}
	}]

	// Group adds a single sub-Group.
	Group: [{
		Name: "A"
		Test: [{
			System: {
				Command: "bash -c"
				Arg: [ "echo Test in group \(Name)"]
			}
		}]
	}]
}

// disable saving of gob data for all Tests
#Test: DataFile: ""

// remove default reporters from all Groups to skip writing any files
#Group: After: []
