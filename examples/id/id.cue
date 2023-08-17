// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example experiments with Test IDs and OutputPrefix.

package id

// Run tests a 2x2 compound ID using a list comprehension.
Run: {
	Serial: [
		for a in [ "X", "Y"]
		for b in [ "1", "2"] {
			{_A: a, _B: b} & _testRun
		},
	]
}

// _testRun is the abstract TestRun.
_testRun: {
	_A: string
	_B: string
	Test: {
		ID: {
			A: _A
			B: _B
		}
		OutputPrefix: "{{.A}}/{{.B}}-"
		System: Command: "echo A=\(_A) B=\(_B)"
	}
}
