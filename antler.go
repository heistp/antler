// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"context"
	"io"
	"log"
	"os"
	"time"

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

// VetCommand loads and checks the CUE config.
type VetCommand struct {
}

// run implements command
func (*VetCommand) run(context.Context) (err error) {
	_, err = LoadConfig(&load.Config{})
	return
}

// RunCommand runs tests and reports.
type RunCommand struct {
	// Filter selects which Tests to run. If Filter is nil, Tests which were not
	// run before or had errors are run.
	Filter TestFilter

	// Skipped is called when a Test was skipped because it wasn't accepted by
	// the Filter.
	Skipped func(*Test)

	// ReRunning is called when a Test is being re-run because the prior result
	// contains errors.
	ReRunning func(*Test)

	// Linked is called when Test data was linked from a prior run.
	Linked func(*Test)

	// Running is called when a Test starts running.
	Running func(*Test)

	// Done is called when the RunCommand is done.
	Done func(RunInfo)
}

// run implements command
func (r RunCommand) run(ctx context.Context) (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	var rw resultRW
	if rw, err = c.Results.open(); err != nil {
		return
	}
	d := doRun{r, rw, &RunInfo{}}
	defer func() {
		d.Info.Elapsed = time.Since(d.Info.Start)
		if d.Info.Ran == 0 {
			if e := rw.Abort(); e != nil && err == nil {
				err = e
			}
		} else {
			var e error
			if d.Info.ResultDir, e = rw.Close(); e != nil && err == nil {
				err = e
			}
		}
		r.Done(*d.Info)
	}()
	d.Info.Start = time.Now()
	err = c.Run.do(ctx, d, reportStack{})
	return
}

// doRun is a doer that runs a Test and its reports.
type doRun struct {
	RunCommand
	RW   resultRW
	Info *RunInfo
}

// RunInfo contains stats and info for a test run.
type RunInfo struct {
	Start     time.Time
	Elapsed   time.Duration
	Ran       int
	Linked    int
	ResultDir string
}

// do implements doer
func (u doRun) do(ctx context.Context, test *Test, rst reportStack) (
	err error) {
	rw := test.RW(u.RW)
	var s reporter
	if u.Filter != nil {
		if !u.Filter.Accept(test) {
			if u.Skipped != nil {
				u.Skipped(test)
			}
			return
		}
	} else if test.DataFile != "" {
		if s, err = u.link(test); err != nil {
			return
		}
		if s != nil {
			var e bool
			if e, err = test.DataHasError(rw); err != nil {
				return
			}
			if e {
				if u.ReRunning != nil {
					u.ReRunning(test)
				}
				s = nil
			} else {
				if u.Linked != nil {
					u.Linked(test)
				}
				u.Info.Linked++
			}
		}
	}
	if s == nil {
		if u.Running != nil {
			u.Running(test)
		}
		if s, err = u.run(ctx, test); err != nil {
			return
		}
		u.Info.Ran++
	}
	err = teeReport(ctx, s, test, rw, rst)
	return
}

// run runs a Test.
func (u doRun) run(ctx context.Context, test *Test) (src reporter, err error) {
	rw := test.RW(u.RW)
	var w io.WriteCloser
	if w, err = test.DataWriter(rw); err != nil {
		if _, ok := err.(NoDataFileError); !ok {
			return
		}
		err = nil
	}
	var a appendData
	var p report = test.During.report()
	if w != nil {
		p = append(p, writeData{w})
	} else {
		p = append(p, &a)
	}
	d := make(chan any, dataChanBufLen)
	ctx, x := context.WithCancelCause(ctx)
	defer x(nil)
	go node.Do(ctx, &test.Run, &exeSource{}, d)
	for e := range p.pipeline(ctx, d, nil, rw) {
		x(e)
		if err == nil {
			err = e
		}
	}
	if err != nil {
		return
	}
	if w != nil {
		var r io.ReadCloser
		if r, err = test.DataReader(rw); err != nil {
			return
		}
		src = readData{r}
	} else {
		src = rangeData(a)
	}
	return
}

// link hard links the DataFile and FileRefs from the prior Test run, and
// returns a source reporter for the report pipeline. If there is no prior Test
// run or DataFile, the returned src and err are both nil.
func (u doRun) link(test *Test) (src reporter, err error) {
	rw := test.RW(u.RW)
	var ok bool
	if ok, err = test.LinkPriorData(rw); err != nil || !ok {
		return
	}
	var r io.ReadCloser
	if r, err = test.DataReader(rw); err != nil {
		return
	}
	src = readData{r}
	return
}

// teeReport runs the Test.Report and reportStack pipelines concurrently, using
// src to supply the data.
func teeReport(ctx context.Context, src reporter, test *Test, rw rwer,
	rst reportStack) (err error) {
	var r []report
	r = append(r, test.Report.report())
	r = append(r, rst.report())
	ctx, x := context.WithCancelCause(ctx)
	defer x(nil)
	for e := range report([]reporter{src}).tee(ctx, rw, nil, r...) {
		x(e)
		if err == nil {
			err = e
		}
	}
	return
}

// ReportCommand runs the After reports using the data files as the source.
type ReportCommand struct {
	// NoDataFile is called when a report was skipped because the Test's
	// DataFile field is empty.
	NoDataFile func(test *Test)

	// NotFound is called when a report was skipped because the data file needed
	// to run it doesn't exist.
	NotFound func(test *Test)

	// Reporting is called when a report starts running.
	Reporting func(test *Test)
}

// run implements command
func (r ReportCommand) run(ctx context.Context) (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	var rw resultRW
	if rw, err = c.Results.open(); err != nil {
		return
	}
	defer func() {
		// TODO add Done func to ReportCommand and set ResultPath
		var e error
		if _, e = rw.Close(); e != nil && err == nil {
			err = e
		}
	}()
	d := doReport{r, rw}
	err = c.Run.do(ctx, d, reportStack{})
	return
}

// doReport is a doer that runs reports.
type doReport struct {
	ReportCommand
	RW resultRW
}

// do implements doer
func (d doReport) do(ctx context.Context, test *Test, rst reportStack) (
	err error) {
	rw := test.RW(d.RW)
	var ok bool
	if ok, err = test.LinkPriorData(rw); err != nil {
		if _, o := err.(NoDataFileError); o {
			if d.NoDataFile != nil {
				d.NoDataFile(test)
			}
			err = nil
		}
		return
	}
	if !ok {
		if d.NotFound != nil {
			d.NotFound(test)
		}
		return
	}
	var r io.ReadCloser
	if r, err = test.DataReader(rw); err != nil {
		return
	}
	err = teeReport(ctx, readData{r}, test, rw, rst)
	return
}

// ServerCommand runs the builtin web server.
type ServerCommand struct {
}

// run implements command
func (s ServerCommand) run(ctx context.Context) (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	log.SetPrefix("")
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	err = c.Server.Run(ctx)
	return
}
