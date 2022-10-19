// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package main

import (
	"fmt"
	"os"

	"github.com/heistp/antler/node"
	"golang.org/x/sys/unix"
)

func main() {
	var err error
	if err = node.TestSockdiag(unix.AF_INET); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	if err = node.TestSockdiag(unix.AF_INET6); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
