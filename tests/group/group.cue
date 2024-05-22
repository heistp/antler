// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

// This Antler test exercises basic Group functionality.

package group

// Test defines a single Test that emits Hello World.
Group: {
	Test: [
		{
			// System is a system command.
			System: {
				Command: "bash -c"
				Arg: [ "echo Test in default group"]
			}
			// disable saving of gob data
			DataFile: ""
			// remove default reporters to skip writing node.log
			AfterDefault: []
		},
	]

	Group: [
		{
			Name: "A"
			Test: [
				{
					System: {
						Command: "bash -c"
						Arg: [ "echo Test in group A"]
					}
					DataFile: ""
					AfterDefault: []
				},
			]
		},
	]
}
