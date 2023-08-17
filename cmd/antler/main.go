// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"github.com/heistp/antler"
	"github.com/heistp/antler/node"
	"github.com/spf13/cobra"
)

// root returns the root cobra command.
func root() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:           "antler",
		Short:         "Active Network Tester of Load et Response",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(list())
	cmd.AddCommand(run())
	cmd.AddCommand(report())
	cmd.AddCommand(vet())
	cmd.Version = antler.Version
	return
}

// list returns the list cobra command.
func list() (cmd *cobra.Command) {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists tests and their output path prefixes",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var c *antler.Config
			if c, err = antler.LoadConfig(&load.Config{}); err != nil {
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Test ID\tOutput Prefix")
			fmt.Fprintln(w, "-------\t-------------")
			c.Run.VisitTests(func(t *antler.Test) bool {
				var p string
				if p, err = t.OutputPath(""); err != nil {
					p = err.Error()
				}
				fmt.Fprintf(w, "%s\t%s\n", t.ID, p)
				return true
			})
			w.Flush()
			return
		},
	}
}

// run returns the run cobra command.
func run() (cmd *cobra.Command) {
	c := node.NewControl()
	r := &antler.RunCommand{
		c,
		false,
		func(test *antler.Test, path string) {
			fmt.Printf("%s already exists, skipping test (use -f to force)\n",
				path)
		},
	}
	cmd = &cobra.Command{
		Use:   "run",
		Short: "Runs tests and reports",
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
	r := &antler.ReportCommand{
		func(test *antler.Test) {
			fmt.Printf("%s was skipped because its DataFile field is empty\n",
				test.ID)
		},
		func(test *antler.Test, path string) {
			fmt.Printf("%s was skipped because '%s' was not found\n",
				test.ID, path)
		},
	}
	return &cobra.Command{
		Use:   "report",
		Short: "Re-runs reports using existing data files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return antler.Run(r)
		},
	}
}

// vet returns the vet cobra command.
func vet() (cmd *cobra.Command) {
	v := &antler.VetCommand{}
	return &cobra.Command{
		Use:   "vet",
		Short: "Checks the CUE configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return antler.Run(v)
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
