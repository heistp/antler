// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

// This Antler test exercises timeouts.

package timeout

// Test contains a single Test that sleeps for 5 seconds, while the timeout
// is 1 second.
Test: [{
	// System is a system command.
	System: Command: "sleep 5"
	// disable saving of gob data
	DataFile: ""
	// remove default reporters to skip writing any files
	AfterDefault: []
	// Timeout sets the test timeout to 1 second, which is less than the sleep.
	Timeout: "1s"
}]
