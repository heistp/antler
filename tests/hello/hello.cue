// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests emitting Hello World in the root node.

package hello

// Run contains a single Test that emits Hello World.
Run: {
	Test: {
		// System is a system command.
		System: {
			Command: "bash -c"
			Args: [ "echo Hello World!"]
		}

		// disable saving of gob data
		DataFile: ""
		// remove default report that writes node.log
		Report: []
	}
}
