// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler test runs a node with environment variables set.

package env

// The Root Group contains a single Test that emits environment variables.
Root: {
	Test: [{
		Child: {
			Node: {
				ID:       "envtest"
				Platform: "linux-amd64"
				Launcher: Local: {}
				Env: Vars: [ "FOO=BAR", "FOO2=BAR2"]
			}
			System: {
				Command: "bash -c"
				Arg: [
					"echo FOO=$FOO FOO2=$FOO2",
				]
			}
		}
		// disable saving of gob data
		DataFile: ""
	}]
	// remove default reporters to skip writing any files
	After: []
}
