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

// root returns the root cobra command.
func root() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:           "antler",
		Short:         "Active Network Tester for Load et Response",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(run())
	cmd.AddCommand(report())
	return
}

// run returns the run cobra command.
func run() (cmd *cobra.Command) {
	c := node.NewControl()
	r := &antler.RunCommand{c, false}
	cmd = &cobra.Command{
		Use:   "run",
		Short: "Runs tests and reports in current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			defer c.Stop()
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
			go func() {
				s := <-sc
				fmt.Fprintf(os.Stderr,
					"%s, canceling (one more to terminate)\n", s)
				c.Cancel(s.String())
				s = <-sc
				fmt.Fprintf(os.Stderr, "%s, exiting forcibly\n", s)
				os.Exit(-1)
			}()
			return antler.Run(r)
		},
	}
	cmd.Flags().BoolVarP(&r.Force, "force", "f", false,
		"force tests to run, overwriting existing results")
	return
}

// report returns the report cobra command.
func report() (cmd *cobra.Command) {
	r := &antler.ReportCommand{}
	return &cobra.Command{
		Use:   "report",
		Short: "Re-runs reports using existing data files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return antler.Run(r)
		},
	}
}

// main executes the antler command.
func main() {
	if err := root().Execute(); err != nil {
		s := err.Error()
		if ce, ok := err.(errors.Error); ok {
			s = errors.Details(ce, nil)
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], s)
		os.Exit(1)
	}
}
