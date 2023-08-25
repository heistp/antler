// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node"
)

// dataChanBufLen is used as the buffer length for data channels.
const dataChanBufLen = 64

// Run runs an Antler Command.
func Run(ctx context.Context, cmd Command) error {
	return cmd.run(ctx)
}

// A Command is an Antler command.
type Command interface {
	run(context.Context) error
}

// RunCommand runs tests and reports.
type RunCommand struct {
	// Force re-runs the test and overwrites any existing data.
	Force bool

	// Filter selects which tests to run.
	Filter TestFilter

	// SkippedFiltered is called when a test was skipped because it was rejected
	// by the Filter.
	SkippedFiltered func(test *Test)

	// SkippedDataFileExists is called when a test was skipped because there's
	// already an output data file for it and RunCommand.Force is false.
	SkippedDataFileExists func(test *Test, path string)
}

// run implements command
func (r *RunCommand) run(ctx context.Context) (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	err = c.Run.do(ctx, r, reportStack{})
	return
}

// do implements doer
func (c *RunCommand) do(ctx context.Context, test *Test, rst reportStack) (
	err error) {
	if c.Filter != nil && !c.Filter.Accept(test) {
		c.SkippedFiltered(test)
		return
	}
	var w io.WriteCloser
	if w, err = test.DataWriter(c.Force); err != nil {
		switch e := err.(type) {
		case *FileExistsError:
			if c.SkippedDataFileExists != nil {
				c.SkippedDataFileExists(test, e.Path)
			}
			err = nil
			return
		case *NoDataFileError:
			err = nil
		default:
			return
		}
	}
	var a appendData
	p := test.During.report()
	if w != nil {
		p = append(p, writeData{w})
	} else {
		p = append(p, &a)
	}
	d := make(chan any, dataChanBufLen)
	ctx, x := context.WithCancelCause(ctx)
	defer x(nil)
	go node.Do(ctx, &test.Run, &exeSource{}, d)
	for e := range p.pipeline(ctx, d, nil, test) {
		x(e)
		if err == nil {
			err = e
		}
	}
	if err != nil {
		return
	}
	var s reporter
	if w != nil {
		var r io.ReadCloser
		if r, err = test.DataReader(); err != nil {
			return
		}
		s = readData{r}
	} else {
		fmt.Fprintf(os.Stderr, "len(a) = %d\n", len(a))
		s = rangeData(a)
	}
	err = doReport(ctx, s, test, rst)
	return
}

// doReport runs the Test.Report and reportStack pipelines concurrently,
// using src to supply the data.
func doReport(ctx context.Context, src reporter, test *Test, rst reportStack) (
	err error) {
	var r []report
	r = append(r, test.Report.report())
	r = append(r, rst.report())
	ctx, x := context.WithCancelCause(ctx)
	for e := range report([]reporter{src}).tee(ctx, test, nil, r...) {
		x(e)
		if err == nil {
			err = e
		}
	}
	return
}

// ReportCommand runs the After reports using the data files as the source.
type ReportCommand struct {
	// Filter selects which tests to run.
	Filter TestFilter

	// SkippedFiltered is called when a report was skipped because it was
	// rejected by the Filter.
	SkippedFiltered func(test *Test)

	// SkippedNoDataFile is called when a report was skipped because the Test's
	// DataFile field is empty.
	SkippedNoDataFile func(test *Test)

	// SkippedNotFound is called when a report was skipped because the data file
	// needed to run it doesn't exist.
	SkippedNotFound func(test *Test, path string)
}

// run implements command
func (r *ReportCommand) run(ctx context.Context) (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	err = c.Run.do(ctx, r, reportStack{})
	return
}

// do implements doer
func (c *ReportCommand) do(ctx context.Context, test *Test, rst reportStack) (
	err error) {
	if c.Filter != nil && !c.Filter.Accept(test) {
		c.SkippedFiltered(test)
		return
	}
	var r io.ReadCloser
	if r, err = test.DataReader(); err != nil {
		if _, ok := err.(*NoDataFileError); ok {
			if c.SkippedNoDataFile != nil {
				c.SkippedNoDataFile(test)
			}
			err = nil
			return
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return
		}
		e := err.(*fs.PathError)
		if c.SkippedNotFound != nil {
			c.SkippedNotFound(test, e.Path)
		}
		err = nil
		return
	}
	err = doReport(ctx, readData{r}, test, rst)
	return
}

// VetCommand loads and checks the CUE config.
type VetCommand struct {
}

// run implements command
func (*VetCommand) run(context.Context) (err error) {
	_, err = LoadConfig(&load.Config{})
	return
}
