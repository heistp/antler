// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler test experiments with Test IDs and Path.

package id

// Test contains a list of Tests generated with a list comprehension.
Test: [
	for a in [ "W", "X", "Y", "Z"]
	for b in [ "1", "2", "3", "4"] {
		// ID is a compound ID with two key/value pairs.
		ID: {A: a, B: b}

		// Path uses a directory for A, and B as the filename prefix.
		Path: "\(a)/\(b)-"

		// Emit a and b, for testing.
		System: Command: "echo a=\(a) b=\(b)"

		// disable saving of gob data
		DataFile: ""

		// remove default reporters to skip writing any files
		After: []
	},
]
