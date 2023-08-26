// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests streaming FileData back from a node.

package env

// Run contains a single Test that streams data from /dev/random to illustrate
// how data streaming works.
//
// Because all data is streamed, it's transferred from the child node to the
// root node as the test runs. We have overridden Test: Report to *not* include
// the SavesFiles reporter, so the data is discarded.
//
// While the test runs, you should see CPU used by antler and antler-node for
// transferring the data, but the heap should stay stable. Be careful if you
// comment out the ResultStream config. The data from /dev/random will be
// buffered in the node's heap, and since there is 6.4G of it, you may run
// short on memory. :)
Run: {
	Test: Serial: [
		// stream everything in root node
		{ResultStream: Include: All: true},
		{Child: {
			Node: {
				ID:       "envtest"
				Platform: "linux-amd64"
				Launcher: Local: {}
			}
			Serial: [
				// stream everything in child node
				{ResultStream: Include: All: true},
				{System: {
					Command: "dd if=/dev/random bs=64K count=100000"
					Stdout:  "discard.bin"
				}},
			]
		}},
	]

	Test: {
		// disable saving of gob data
		DataFile: ""
		// remove default reporter that writes node.log
		Report: []
	}
}
