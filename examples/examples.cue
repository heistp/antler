// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

// This package contains Antler examples. By keeping the examples in one CUE
// package, common setup can be shared.

package examples

// Test lists the examples Tests.
Test: [
	_tcpstream,
	_ratedrop,
	_iperf3,
	_packets,
	_vbrudp,
	_fct,
	_tcpinfo,
]

// MultiReport adds an HTML index file.
MultiReport: [{
	Index: {
		Title: "Antler Examples"
	}
}]
