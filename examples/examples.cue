// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This package contains Antler examples. By keeping the examples in one CUE
// package, common setup can be shared.

package examples

// Run lists the example Tests to run. We run the tests in serial, but
// changing Serial to Parallel allows tests to be run concurrently.
Run: {
	Serial: [
		_tcpstream,
	]
}
