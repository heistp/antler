// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2024 Pete Heist

// This Antler test runs a node via ssh.
//
// To customize the host the test is run on, either change the default for
// _dest, or create a file e.g. local.cue with "_dest: hostname".
//
// To test sudo support, set the Root field for the SSH Launcher to true.

package ssh

_dest: string | *"localhost"

// Test contains a single Test that runs a command on an ssh node.
Test: [{
	Child: {
		Node: {
			ID:       _dest
			Platform: "linux-amd64"
			Launcher: SSH: {Root: false}
		}
		System: {
			Command: "bash -c"
			Arg: [
				"echo $(whoami) says hello on $(hostname)",
			]
		}
	}
	// disable saving of gob data
	DataFile: ""
	// remove default reporters to skip writing any files
	AfterDefault: []
	// enable HMAC protection
	HMAC: true
}]
