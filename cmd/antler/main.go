// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package main

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"github.com/heistp/antler"
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
	cmd.AddCommand(vet())
	cmd.AddCommand(list())
	cmd.AddCommand(run())
	cmd.AddCommand(report())
	cmd.AddCommand(server())
	cmd.Version = antler.Version
	return
}

// vet returns the vet cobra command.
func vet() (cmd *cobra.Command) {
	c := context.Background()
	v := &antler.VetCommand{}
	return &cobra.Command{
		Use:   "vet",
		Short: "Checks the CUE configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return antler.Run(c, v)
		},
	}
}

// list returns the list cobra command.
func list() (cmd *cobra.Command) {
	return &cobra.Command{
		Use:   "list [filter] ...",
		Short: "Lists tests and their result path prefixes",
		Long: help(`List lists tests and their result path prefixes.

{{template "filter" "list"}}
`),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var f antler.TestFilter = antler.BoolFilter(true)
			if len(args) > 0 {
				if f, err = newRegexFilter(args); err != nil {
					return
				}
			}
			var c *antler.Config
			if c, err = antler.LoadConfig(&load.Config{}); err != nil {
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Test ID\tResult Prefix")
			fmt.Fprintln(w, "-------\t-------------")
			c.Run.VisitTests(func(t *antler.Test) bool {
				if !f.Accept(t) {
					return true
				}
				fmt.Fprintf(w, "%s\t%s\n", t.ID, t.ResultPrefixX)
				return true
			})
			w.Flush()
			return
		},
	}
}

// run returns the run cobra command.
func run() (cmd *cobra.Command) {
	r := &antler.RunCommand{
		Filter: nil,
		Skipped: func(test *antler.Test) {
			fmt.Printf("skipped %s\n", test.ID)
		},
		ReRunning: func(test *antler.Test) {
			fmt.Printf("re-running %s due to prior error\n", test.ID)
		},
		Running: func(test *antler.Test) {
			fmt.Printf("running %s...\n", test.ID)
		},
		Linked: func(test *antler.Test) {
			fmt.Printf("linked %s\n", test.ID)
		},
		Done: func(info antler.RunInfo) {
			fmt.Printf("ran %d tests, linked %d, elapsed %s\n",
				info.Ran, info.Linked, info.Elapsed)
			if info.ResultDir == "" {
				fmt.Printf("no tests run, result not saved\n")
			} else {
				fmt.Printf("result saved to: '%s'\n", info.ResultDir)
			}
		},
	}
	var a bool
	cmd = &cobra.Command{
		Use:   "run [filter] ...",
		Short: "Runs tests and reports",
		Long: help(`Run runs tests and reports.

{{template "filter" "run"}}
`),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			c, x := context.WithCancelCause(context.Background())
			defer x(nil)
			if a && len(args) > 0 {
				err = errors.New("-a/--all not compatible with arguments")
				return
			}
			if len(args) > 0 {
				if r.Filter, err = newRegexFilter(args); err != nil {
					return
				}
			}
			if a {
				r.Filter = antler.BoolFilter(true)
			}
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
			go func() {
				s := <-sc
				fmt.Fprintf(os.Stderr,
					"%s, canceling (one more to terminate)\n", s)
				x(errors.New(s.String()))
				s = <-sc
				fmt.Fprintf(os.Stderr, "%s, exiting forcibly\n", s)
				os.Exit(-1)
			}()
			err = antler.Run(c, r)
			return
		},
	}
	cmd.Flags().BoolVarP(&a, "all", "a", false,
		"runs all tests (may not be used with filter args)")
	return
}

// report returns the report cobra command.
func report() (cmd *cobra.Command) {
	r := &antler.ReportCommand{
		Filter: nil,
		Skipping: func(test *antler.Test) {
			fmt.Printf("skipping %s\n", test.ID)
		},
		Reporting: func(test *antler.Test) {
			fmt.Printf("running reports for %s\n", test.ID)
		},
		NoDataFile: func(test *antler.Test) {
			fmt.Printf("skipping %s, DataFile field is empty\n", test.ID)
		},
		NotFound: func(test *antler.Test, path string) {
			fmt.Printf("skipping %s, '%s' not found\n", test.ID, path)
		},
	}
	return &cobra.Command{
		Use:   "report [filter] ...",
		Short: "Re-runs reports using existing data files",
		Long: help(`Report re-runs reports using existing data files.

{{template "filter" "report"}}
`),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			c, x := context.WithCancelCause(context.Background())
			defer x(nil)
			if len(args) > 0 {
				if r.Filter, err = newRegexFilter(args); err != nil {
					return
				}
			}
			err = antler.Run(c, r)
			return
		},
	}
}

// server returns the server cobra command.
func server() (cmd *cobra.Command) {
	s := &antler.ServerCommand{}
	return &cobra.Command{
		Use:   "server",
		Short: "Runs the builtin web server",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			c, x := context.WithCancelCause(context.Background())
			defer x(nil)
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
			go func() {
				s := <-sc
				fmt.Fprintf(os.Stderr,
					"%s, shutting down (one more to terminate)\n", s)
				x(errors.New(s.String()))
				s = <-sc
				fmt.Fprintf(os.Stderr, "%s, exiting forcibly\n", s)
				os.Exit(-1)
			}()
			err = antler.Run(c, s)
			return
		},
	}
}

// newRegexFilter returns a TestFilter that's a logical and of the given
// regex filters.
func newRegexFilter(args []string) (flt antler.AndFilter, err error) {
	for _, a := range args {
		var f antler.TestFilter
		if f, err = antler.NewRegexFilterArg(a); err != nil {
			return
		}
		flt = append(flt, f)
	}
	return
}

// helpTemplate contains defined templates for common help snippets.
const helpTemplate = `
{{- define "filter" -}}
Each filter argument may be either a single regex pattern matching the value of
any ID field, or a string in the form key=value, where key and value are
separate patterns that must match both a Test ID key and value for it to be
accepted. Multiple filters are combined together with a logical AND.

Example 1: antler {{.}} cca=cubic

Example 2: antler {{.}} qdisc=codel rtt='(20ms|40ms)'
{{end}}
`

// help executes the given template text together with helpTemplate and returns
// the result as a string.
func help(text string) string {
	t := template.Must(template.New("help").Parse(helpTemplate))
	t = template.Must(t.Parse(text))
	var h strings.Builder
	if e := t.Execute(&h, nil); e != nil {
		panic(e)
	}
	return h.String()
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
