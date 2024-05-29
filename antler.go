// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"sync"
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

// Run2 runs an Antler Command.
func Run2(ctx context.Context, cmd Command) error {
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
		if r.Done != nil {
			r.Done(*d.Info)
		}
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
	sync.Mutex
	Start     time.Time
	Elapsed   time.Duration
	Ran       int
	Linked    int
	ResultDir string
}

// ran increments the Ran field.
func (i *RunInfo) ran() {
	i.Lock()
	i.Ran++
	i.Unlock()
}

// linked increments the Linked field.
func (i *RunInfo) linked() {
	i.Lock()
	i.Linked++
	i.Unlock()
}

// do implements doer
func (u doRun) do(ctx context.Context, test *Test, rst reportStack) (
	err error) {
	rw := test.RW(u.RW)
	var s reporter
	if u.Filter != nil {
		if !u.Filter.Accept(test) {
			if s, err = u.link(test); err != nil {
				return
			}
			if s == nil {
				if u.Skipped != nil {
					u.Skipped(test)
				}
				return
			} else {
				if u.Linked != nil {
					u.Linked(test)
				}
				u.Info.linked()
			}
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
				u.Info.linked()
			}
		}
	}
	if s == nil {
		if u.Running != nil {
			u.Running(test)
		}
		u.Info.ran()
		if s, err = u.run(ctx, test); err != nil {
			return
		}
	}
	err = teeReport(ctx, s, test, rw, rst)
	return
}

// run runs a Test.
func (u doRun) run(ctx context.Context, test *Test) (src reporter, err error) {
	rw := test.RW(u.RW)
	var w io.WriteCloser
	if w, err = test.DataWriter(rw); err != nil {
		if _, ok := err.(DataFileUnsetError); !ok {
			return
		}
		err = nil
	}
	var a appendData
	var p report = test.DuringDefault.report()
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
	if err = test.LinkPriorData(rw); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
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
	r = append(r, test.ReportDefault.report())
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

// Run2Command runs tests and reports.
type Run2Command struct {
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
func (r Run2Command) run(ctx context.Context) (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	var rw resultRW
	if rw, err = c.Results.open(); err != nil {
		return
	}
	d := doRun2{r, rw, &RunInfo{}}
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
		if r.Done != nil {
			r.Done(*d.Info)
		}
	}()
	d.Info.Start = time.Now()
	err = c.Group.do(ctx, d)
	return
}

// doRun2 is a doer that runs a Test and its reports.
type doRun2 struct {
	Run2Command
	RW   resultRW
	Info *RunInfo
}

// do implements doer
func (u doRun2) do(ctx context.Context, test *Test) (
	err error) {
	rw := test.RW(u.RW)
	var s reporter
	if u.Filter != nil {
		if !u.Filter.Accept(test) {
			if s, err = u.link(test); err != nil {
				return
			}
			if s == nil {
				if u.Skipped != nil {
					u.Skipped(test)
				}
				return
			} else {
				if u.Linked != nil {
					u.Linked(test)
				}
				u.Info.linked()
			}
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
				u.Info.linked()
			}
		}
	}
	if s == nil {
		if u.Running != nil {
			u.Running(test)
		}
		u.Info.ran()
		if s, err = u.run(ctx, test); err != nil {
			return
		}
	}
	r := report([]reporter{s}).add(test.Group.After.report())
	for e := range r.pipeline(ctx, nil, nil, rw) {
		if err == nil {
			err = e
		}
	}
	return
}

// run runs a Test.
func (u doRun2) run(ctx context.Context, test *Test) (src reporter, err error) {
	rw := test.RW(u.RW)
	var w io.WriteCloser
	if w, err = test.DataWriter(rw); err != nil {
		if _, ok := err.(DataFileUnsetError); !ok {
			return
		}
		err = nil
	}
	var a appendData
	var p report = test.Group.During.report()
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
func (u doRun2) link(test *Test) (src reporter, err error) {
	rw := test.RW(u.RW)
	if err = test.LinkPriorData(rw); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return
	}
	var r io.ReadCloser
	if r, err = test.DataReader(rw); err != nil {
		return
	}
	src = readData{r}
	return
}

// ReportCommand runs the After reports using the data files as the source.
type ReportCommand struct {
	// DataFileUnset is called when a report was skipped because the Test's
	// DataFile field is empty.
	DataFileUnset func(test *Test)

	// NotFound is called when a report was skipped because the data file needed
	// to run it doesn't exist.
	NotFound func(test *Test, name string)

	// Reporting is called when a report starts running.
	Reporting func(test *Test)

	// Done is called when the ReportCommand is done.
	Done func(ReportInfo)
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
	d := doReport{r, rw, &ReportInfo{}}
	defer func() {
		var e error
		if _, e = rw.Close(); e != nil && err == nil {
			err = e
		}

		d.Info.Elapsed = time.Since(d.Info.Start)
		if d.Info.Reported == 0 {
			if e := rw.Abort(); e != nil && err == nil {
				err = e
			}
		} else {
			var e error
			if d.Info.ResultDir, e = rw.Close(); e != nil && err == nil {
				err = e
			}
		}
		if r.Done != nil {
			r.Done(*d.Info)
		}
	}()
	d.Info.Start = time.Now()
	err = c.Run.do(ctx, d, reportStack{})
	return
}

// doReport is a doer that runs reports.
type doReport struct {
	ReportCommand
	RW   resultRW
	Info *ReportInfo
}

// ReportInfo contains stats and info for a report run.
type ReportInfo struct {
	Start     time.Time
	Elapsed   time.Duration
	Reported  int
	ResultDir string
}

// do implements doer
func (d doReport) do(ctx context.Context, test *Test, rst reportStack) (
	err error) {
	rw := test.RW(d.RW)
	if err = test.LinkPriorData(rw); err != nil {
		switch e := err.(type) {
		case DataFileUnsetError:
			if d.DataFileUnset != nil {
				d.DataFileUnset(test)
			}
			err = nil
		case LinkError:
			if d.NotFound != nil {
				d.NotFound(test, e.Name)
			}
			err = nil
		}
		return
	}
	if d.Reporting != nil {
		d.Reporting(test)
	}
	var r io.ReadCloser
	if r, err = test.DataReader(rw); err != nil {
		return
	}
	d.Info.Reported++
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
