// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"context"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node"
)

// dataChanBufLen is used as the buffer length for data channels.
const dataChanBufLen = 64

//go:embed init/*.cue
var initCue embed.FS

// Run runs an Antler Command.
func Run(ctx context.Context, cmd Command) error {
	return cmd.run(ctx)
}

// A Command is an Antler command.
type Command interface {
	run(context.Context) error
}

// InitCommand creates a new test package in the current directory.
type InitCommand struct {
	Package string // package name, or empty for parent directory name

	// WritingPackage is called before the package is written.
	WritingPackage func(pkg string)

	// WrotePackage is called after the package was written.
	WrotePackage func(pkg string)

	// WritingFile is called before a package file is written.
	WritingFile func(name string)

	// WroteFile is called after a package file was written.
	WroteFile func(name string)
}

// run implements command
func (c *InitCommand) run(context.Context) (err error) {
	// return an error if cwd isn't empty
	var d *os.File
	if d, err = os.Open("."); err != nil {
		return
	}
	defer d.Close()
	if _, err = d.Readdirnames(1); err == nil {
		err = fmt.Errorf("current directory must be empty")
	}
	if err != io.EOF {
		return
	}
	err = nil

	// determine package name if not set
	if c.Package == "" {
		var d string
		if d, err = os.Getwd(); err != nil {
			return
		}
		c.Package = validIdentifier(filepath.Base(d))
	}

	// write template tree locally
	var s fs.FS
	if s, err = fs.Sub(initCue, "init"); err != nil {
		return
	}
	if c.WritingPackage != nil {
		c.WritingPackage(c.Package)
	}
	w := func(path string, d fs.DirEntry, e error) (err error) {
		if e != nil {
			err = e
			return
		}
		if d.IsDir() {
			return
		}
		var t *template.Template
		if t, err = template.ParseFS(s, path); err != nil {
			return
		}
		n := path
		if n == "init.cue" {
			n = c.Package + ".cue"
		}
		if c.WritingFile != nil {
			c.WritingFile(n)
		}
		var f *os.File
		if f, err = os.Create(n); err != nil {
			return
		}
		defer f.Close()
		if err = t.Execute(f, c); err == nil && c.WroteFile != nil {
			c.WroteFile(n)
		}
		return
	}
	if err = fs.WalkDir(s, ".", w); err != nil {
		return
	}
	if c.WrotePackage != nil {
		c.WrotePackage(c.Package)
	}
	return
}

// validIdentifier returns a valid Go identifier for the given string.
func validIdentifier(s string) string {
	// Remove any leading or trailing whitespace
	s = strings.TrimSpace(s)

	// Replace non-alphanumeric characters (except underscores) with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	s = re.ReplaceAllString(s, "_")

	// Return a default identifier for an empty string
	if len(s) == 0 {
		return "_"
	}

	// Ensure the identifier starts with a letter or underscore
	if !unicode.IsLetter(rune(s[0])) && s[0] != '_' {
		s = "_" + s
	}

	return s
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
	m := newMultiRunner(c.MultiReport)
	d := doRun{r, rw, m, &RunInfo{}}
	defer func() {
		if e := m.stop(rw); e != nil && err == nil {
			err = e
		}
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
	if err = m.start(rw); err != nil {
		return
	}
	d.Info.Start = time.Now()
	for _, t := range c.Test {
		t := t
		if err = d.Test(ctx, &t); err != nil {
			return
		}
	}
	return
}

// doRun is a Tester that runs a Test and its reports.
type doRun struct {
	RunCommand
	RW    resultRW
	Multi *multiRunner
	Info  *RunInfo
}

// Test implements Tester.
func (d doRun) Test(ctx context.Context, test *Test) (err error) {
	rw := test.RW(d.RW)
	var s reporter
	if d.Filter != nil {
		if !d.Filter.Accept(test) {
			if s, err = d.link(test); err != nil {
				return
			}
			if s == nil {
				if d.Skipped != nil {
					d.Skipped(test)
				}
				return
			} else {
				if d.Linked != nil {
					d.Linked(test)
				}
				d.Info.linked()
			}
		}
	} else if test.DataFile != "" {
		if s, err = d.link(test); err != nil {
			return
		}
		if s != nil {
			var e bool
			if e, err = test.DataHasError(rw); err != nil {
				return
			}
			if e {
				if d.ReRunning != nil {
					d.ReRunning(test)
				}
				s = nil
			} else {
				if d.Linked != nil {
					d.Linked(test)
				}
				d.Info.linked()
			}
		}
	}
	if s == nil {
		if d.Running != nil {
			d.Running(test)
		}
		d.Info.ran()
		if s, err = d.run(ctx, test); err != nil {
			return
		}
	}
	r := report([]reporter{s})
	r = r.add(test.AfterDefault.report())
	r = r.add(test.After.report())
	o, me := d.Multi.tee(ctx, rw, test)
	pe := r.pipeline(ctx, rw, nil, o)
	for e := range mergeErr(me, pe) {
		if err == nil {
			err = e
		}
	}
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
	p := test.DuringDefault.report()
	p = p.add(test.During.report())
	if w != nil {
		p = append(p, writeData{w})
	} else {
		p = append(p, &a)
	}
	d := make(chan any, dataChanBufLen)
	ctx, x := context.WithCancelCause(ctx)
	defer x(nil)
	if test.Timeout > 0 {
		var t context.CancelFunc
		ctx, t = context.WithTimeout(ctx, test.Timeout.Duration())
		defer t()
	}
	go node.Do(ctx, &test.Run, &exeSource{}, d)
	for e := range p.pipeline(ctx, rw, d, nil) {
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

// ReportInfo contains stats and info for a report run.
type ReportInfo struct {
	Start     time.Time
	Elapsed   time.Duration
	Reported  int
	ResultDir string
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
	m := newMultiRunner(c.MultiReport)
	d := doReport{r, rw, m, &ReportInfo{}}
	defer func() {
		if e := m.stop(rw); e != nil && err == nil {
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
	if err = m.start(rw); err != nil {
		return
	}
	d.Info.Start = time.Now()
	for _, t := range c.Test {
		t := t
		if err = d.Test(ctx, &t); err != nil {
			return
		}
	}
	return
}

// doReport is a Tester that runs reports.
type doReport struct {
	ReportCommand
	RW    resultRW
	Multi *multiRunner
	Info  *ReportInfo
}

// Test implements Tester.
func (d doReport) Test(ctx context.Context, test *Test) (err error) {
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
	t := report([]reporter{readData{r}})
	t = t.add(test.AfterDefault.report())
	t = t.add(test.After.report())
	o, me := d.Multi.tee(ctx, rw, test)
	pe := t.pipeline(ctx, rw, nil, o)
	for e := range mergeErr(me, pe) {
		if err == nil {
			err = e
		}
	}
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

/*
// teeReport runs the Test.Report and reportStack pipelines concurrently, using
// src to supply the data.
//
// NOTE this was used in the reportStack era, and may be removed after some time
// if tee'ing reports is no longer needed
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
*/
