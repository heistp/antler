// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/heistp/antler/node"
)

// main executes the antler-node command.
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "%s: exactly one argument required (node ID)\n")
		fmt.Fprintf(os.Stderr, "usage: %s <node ID>\n", os.Args[0])
		os.Exit(1)
	}
	n := os.Args[1]
	ct := node.NewControl()
	i := make(chan os.Signal, 1)
	signal.Notify(i, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-i
		fmt.Fprintf(os.Stderr, "%s, canceling\n", s)
		ct.Cancel(s.String())
	}()
	c := node.StdioConn()
	if err := node.Serve(n, ct, c); err != nil {
		fmt.Fprintf(os.Stderr, "node exiting with status 1: %s\n", err)
		os.Exit(1)
	}
}
