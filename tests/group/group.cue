// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

// This Antler test exercises basic Group functionality.

package group

// The Root Group contains a single Test, and a sub-Group with another Test.
Root: {
	// Test lists the Tests in the default Group.
	Test: [{
		// System is a system command.
		System: {
			Command: "bash -c"
			Arg: [ "echo Test in default group"]
		}
	}]

	// Sub adds a single sub-Group with another Test.
	Sub: [{
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

// remove default reporters from all Tests to skip writing any files
#Test: #After: []
