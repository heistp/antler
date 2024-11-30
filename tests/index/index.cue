// SPDX-License-Identifier: GPL-3.0-or-later
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

		// SysInfo gathers system information.
		SysInfo: {
			OS: {
				Command: {Command: "uname -a"}
			}
			Command: [
				{Command: "echo a=\(a) b=\(b)"},
			]
		}
	},
]

// MultiReport lists the index report.
MultiReport: [
	{
		Index: {
			Title:   "All Tests, Group By Field A"
			GroupBy: "A"
		}
	},
	{
		ID: {A: "W"}
		Index: {
			Title: "Tests With A=W"
			To:    "W/index.html"
		}
	},
	{
		ID: {A: "X"}
		Index: {
			Title: "Tests With A=X"
			To:    "X/index.html"
		}
	},
]
