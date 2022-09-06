// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cuelang.org/go/cue/errors"
	"github.com/heistp/antler"
	"github.com/heistp/antler/node"
	"github.com/spf13/cobra"
)

// runCmd returns the run cobra command.
func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Runs antler tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctrl := node.NewControl()
			i := make(chan os.Signal, 1)
			signal.Notify(i, os.Interrupt, syscall.SIGTERM)
			go func() {
				s := <-i
				fmt.Fprintf(os.Stderr,
					"%s, canceling (one more to terminate)\n", s)
				ctrl.Cancel(s.String())
				s = <-i
				fmt.Fprintf(os.Stderr, "%s, exiting forcibly\n", s)
				os.Exit(-1)
			}()
			return antler.Run(ctrl)
		},
	}
}

// rootCmd returns the root cobra command.
func rootCmd() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:           "antler",
		Short:         "Active Network Tester for Load et Response",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(runCmd())
	return
}

// main executes the antler command.
func main() {
	if err := rootCmd().Execute(); err != nil {
		msg := err.Error()
		if cueerr, ok := err.(errors.Error); ok {
			msg = errors.Details(cueerr, nil)
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], msg)
		os.Exit(1)
	}
}
