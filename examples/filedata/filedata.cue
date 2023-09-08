// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests streaming FileData back from a node.

package env

// Run contains a single Test that streams data from /dev/random and /dev/zero
// to illustrate how data streaming and compression works.
//
// Because all data is streamed, it's transferred from the child node to the
// root node as the test runs. We have set Test.DataFile to "", so the raw data
// is discarded.
//
// While the test runs, you should see CPU used by antler and antler-node for
// transferring the data, but the heap should stay stable. Be careful if you
// comment out the ResultStream config. The data from /dev/random will be
// buffered in the node's heap, and since there is 640MB of it, you may run
// short on memory. :)
//
// The compression format is chosen based on the file extension. Here, we use
// the .zst extension, so the zstd utility must be present.
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
					Command: "dd if=/dev/random bs=64K count=10000"
					Stdout:  "random.bin"
				}},
				{System: {
					Command: "dd if=/dev/zero bs=64K count=10000"
					Stdout:  "zero.foo"
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
