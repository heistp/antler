// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example experiments with Test IDs and OutputPrefix.

package id

// Run creates a Serial list of TestRuns with a list comprehension.
Run: {
	Serial: [
		for a in [ "W", "X", "Y", "Z"]
		for b in [ "1", "2", "3", "4"] {
			{_A: a, _B: b} & _testRun
		},
	]
}

// _testRun is the abstract TestRun.
_testRun: {
	_A: string
	_B: string
	Test: {
		// ID is a compound ID with two key/value pairs.
		ID: {
			A: _A
			B: _B
		}

		// OutputPrefix uses a directory for A, and B as the filename prefix.
		OutputPrefix: "{{.A}}/{{.B}}-"

		// Emit A and B, for testing.
		System: Command: "echo A=\(_A) B=\(_B)"
	}
}
