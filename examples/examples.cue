// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This package contains Antler examples. By keeping the examples in one CUE
// package, common setup can be shared.

package examples

// The default Group contains no Tests itself, just sub-Groups for each example.
Group: {
	Group: [
		_tcpstream,
		_ratedrop,
		_iperf3,
		_vbrudp,
		_fct,
	]
}
