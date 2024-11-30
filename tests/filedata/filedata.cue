// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

// This Antler test streams FileData back from a node.

package filedata

// Test contains a single Test that streams data from /dev/random and /dev/zero
// to illustrate how data streaming and compression works.
//
// Because all data is streamed, it's transferred from the child node to the
// root node as the test runs. We set Test.DataFile to "", so the raw test data
// is discarded, but the generated data files will remain.
//
// The compression format is chosen based on the file extension. Here, we use
// .zst and .gz, so the zstd and gzip utilities must be present.
Test: [{
	Serial: [
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
				// first, transfer 640K of random data
				{System: {
					Command: "dd if=/dev/random bs=64K count=10"
					Stdout:  "random.bin"
				}},
				// next, transfer and compress 640K of zeroes and transparently
				// save in zstd format
				{System: {
					Command: "dd if=/dev/zero bs=64K count=10"
					Stdout:  "zero.zst"
				}},
			]
		}},
	]
	// add report to compress random.bin to random.bin.gz after the fact.
	//
	// Since we set Destructive to true, random.bin is removed, leaving only
	// random.bin.gz.
	After: [
		{Encode: {
			File: ["random.bin"]
			Extension:   ".gz"
			Destructive: true
		}},
	]
}]

// disable saving of gob data for all Tests
#Test: DataFile: ""
