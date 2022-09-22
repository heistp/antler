// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/heistp/antler/node"
)

// DataFile is the fixed name of the gob-encoded data file containing all the
// results.
const DataFile = "data.gob"

// Run runs an Antler Command, e.g. RunCommand and ReportCommand.
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
	Control *node.Control

	// Force re-runs the test and overwrites any existing data.
	Force bool
}

// run implements command
func (c *RunCommand) run() error {
	return do(c)
}

// dataChanBufSize is used as the buffer size for data channels.
const dataChanBufSize = 64

// do implements doer
func (c *RunCommand) do(test *Test, rst reporterStack) (err error) {
	g := test.outPath(DataFile)
	var ok bool
	if ok, err = c.check(g); err != nil {
		return
	}
	if !ok {
		fmt.Printf("%s already exists, skipping test (use -f to force)\n", g)
		return
	}
	d := make(chan interface{}, dataChanBufSize)
	rst.push([]reporter{&saveData{g}})
	defer func() {
		if e := rst.pop(); e != nil && err == nil {
			err = e
		}
	}()
	go node.Do(&test.Run, &exeSource{}, c.Control, d)
	err = rst.tee(d, test, c.Control)
	return
}

// check determines if a test should or should not be run for the given named
// data file. The ok return parameter is true if it should be run, false if it
// should not be run because the data file already exists, and an error if an
// error occurred while checking.
func (c *RunCommand) check(name string) (ok bool, err error) {
	if c.Force {
		ok = true
		return
	}
	if _, err = os.Stat(name); err != nil && errors.Is(err, os.ErrNotExist) {
		ok = true
		err = nil
	}
	return
}

// ReportCommand runs reports.
type ReportCommand struct {
}

// run implements command
func (c *ReportCommand) run() error {
	return do(c)
}

// do implements doer
func (*ReportCommand) do(test *Test, rst reporterStack) (err error) {
	g := test.outPath(DataFile)
	var f *os.File
	if f, err = os.Open(g); err != nil {
		return
	}
	defer f.Close()
	d := make(chan interface{}, dataChanBufSize)
	go func() {
		var e error
		defer func() {
			if e != nil && e != io.EOF {
				d <- e
			}
			defer close(d)
		}()
		dc := gob.NewDecoder(f)
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
