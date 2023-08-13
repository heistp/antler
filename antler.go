// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"encoding/gob"
	"errors"
	"io"
	"io/fs"

	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node"
)

// dataChanBufSize is used as the buffer size for data channels.
const dataChanBufSize = 64

// Run runs an Antler Command.
func Run(cmd Command) error {
	return cmd.run()
}

// A Command is an Antler command.
type Command interface {
	run() error
}

// RunCommand runs tests and reports.
type RunCommand struct {
	// Control is used to send node control signals.
	Control node.Control

	// Force re-runs the test and overwrites any existing data.
	Force bool

	// Skipped is called when a test was skipped because there's already an
	// output data file for it and RunCommand.Force is false.
	Skipped func(test *Test, path string)
}

// run implements command
func (r *RunCommand) run() (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	err = c.Run.do(r, reporterStack{})
	return
}

// do implements doer
func (c *RunCommand) do(test *Test, rst reporterStack) (err error) {
	var w io.WriteCloser
	if w, err = test.DataWriter(c.Force); err != nil {
		switch e := err.(type) {
		case *FileExistsError:
			if c.Skipped != nil {
				c.Skipped(test, e.Path)
			}
			err = nil
			return
		case *NoDataFileError:
			err = nil
		default:
			return
		}
	}
	if w != nil {
		rst.push([]reporter{saveData{w}})
	}
	d := make(chan interface{}, dataChanBufSize)
	defer rst.pop()
	go node.Do(&test.Run, &exeSource{}, c.Control, d)
	err = rst.tee(d, test, &c.Control)
	return
}

// ReportCommand runs reports.
type ReportCommand struct {
	// SkippedNoDataFile is called when a report was skipped because the Test's
	// DataFile field is empty.
	SkippedNoDataFile func(test *Test)

	// SkippedNotFound is called when a report was skipped because the data file
	// needed to run it doesn't exist.
	SkippedNotFound func(test *Test, path string)
}

// run implements command
func (r *ReportCommand) run() (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	err = c.Run.do(r, reporterStack{})
	return
}

// do implements doer
func (c *ReportCommand) do(test *Test, rst reporterStack) (err error) {
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
	defer r.Close()
	d := make(chan interface{}, dataChanBufSize)
	go func() {
		var e error
		defer func() {
			if e != nil && e != io.EOF {
				d <- e
			}
			defer close(d)
		}()
		dc := gob.NewDecoder(r)
		var a interface{}
		for {
			if e = dc.Decode(&a); e != nil {
				return
			}
			d <- a
		}
	}()
	err = rst.tee(d, test, nil)
	return
}

// VetCommand loads and checks the CUE config.
type VetCommand struct {
}

// run implements command
func (*VetCommand) run() (err error) {
	_, err = LoadConfig(&load.Config{})
	return
}
