// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

package examples

// _dark2 is the Dark2 qualitative color scheme from colorbrewer2.org, with
// the first blue-green color replaced by the green color, as it's too close.
_dark2: [
	//"#1b9e77",
	"#66a61e",
	"#d95f02",
	"#7570b3",
	"#e7298a",
	"#e6ab02",
	"#a6761d",
	"#666666",
]

// tcpdump defines a Runner to run tcpdump to stdout, and save stdout to a file.
_tcpdump: {
	// iface defines the interface to capture on.
	_iface: string & !=""

	// snaplen is how many bytes to capture from each packet.
	_snaplen: int | *128

	System: {
		Command:    "tcpdump -i \(_iface) -s \(_snaplen) -w -"
		Background: true
		Stdout:     "\(_iface).pcap.zst"
	}
}

// Run all commands as root using sudo, if antler was not run as root.
#System: Root: true
