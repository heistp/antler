// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests streaming FileData back from a node.

package env

// Run contains a single Test that streams data from /dev/random.
Run: {
	Test: Serial: [
		{ResultStream: Include: File: [".*"]},
		{Child: {
			Node: {
				ID:       "envtest"
				Platform: "linux-amd64"
				Launcher: Local: {}
				//Env: Vars: [ "GOMEMLIMIT=2MiB"]
			}
			Serial: [
				{ResultStream: Include: File: [".*"]},
				{System: {
					Command: "bash -c"
					Args: [
						"dd if=/dev/random bs=1M count=10",
					]
					Stdout: "rand.bin"
				}},
			]
		}},
	]

	Test: {
		// disable saving of gob data
		DataFile: ""
		// override default report to only emit to stdout
		Report: [
			{EmitLog: {To: ["-"]}},
		]
	}
}
