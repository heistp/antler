// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

// This Antler test generates an index page.

package index

// Test contains a list of Tests generated with a list comprehension.
Test: [
	for a in [ "W", "X"]
	for b in [ "1", "2"] {
		// ID is a compound ID with two key/value pairs.
		ID: {A: a, B: b}

		// Path uses a directory for A, and B as the filename prefix.
		Path: "\(a)/\(b)-"

		// Emit a and b, for testing.
		System: Command: "echo a=\(a) b=\(b)"
	},
]

// MultiReport lists the index report.
MultiReport: [{
	Index: {
		GroupBy: "A"
	}
}]