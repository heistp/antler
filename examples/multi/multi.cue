// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a netns dumbbell,
// and runs two TCP streams from the left to the right endpoint. The middlebox
// (mid namespace) has the cake qdisc added at 50 Mbit.

package multi

// Run contains two Tests.
Run: {
	Parallel: [ for s in [ "4s", "5s", "6s", "7s"] {{_sleep: s} & #myRun}]
}

#myRun: {
	_sleep: string
	Test: {
		OutPath: "\(_sleep)"
		Serial: [
			{Sleep:           _sleep},
			{System: Command: "echo \(_sleep)"},
		]
	}
	Report: [
		{EmitLog: {To: ["node.log", "-"]}},
	]
}
