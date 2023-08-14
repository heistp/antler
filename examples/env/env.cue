// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests running a node with environment variables set.

package env

// Run contains a single Test that emits environment variables.
Run: {
	Test: Child: {
		Node: {
			ID:       "envtest"
			Platform: "linux-amd64"
			Launcher: Local: {}
			Env: Vars: [ "FOO=BAR", "FOO2=BAR2"]
		}
		System: {
			Command: "bash -c"
			Args: [
				"echo FOO=$FOO FOO2=$FOO2",
			]
		}
	}

	Test: {
		// disable saving of gob data
		DataFile: ""
		// override default report to only emit to stdout
		Report: [
			{EmitLog: {To: ["-"]}},
		]
	}
}
