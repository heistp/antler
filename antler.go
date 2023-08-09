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

	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node"
)

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

// dataChanBufSize is used as the buffer size for data channels.
const dataChanBufSize = 64

// do implements doer
func (c *RunCommand) do(test *Test, rst reporterStack) (err error) {
	var g string
	if test.DataFile != "" {
		g = test.outPath(test.DataFile)
		var ok bool
		if ok, err = c.check(g); err != nil {
			return
		}
		if !ok {
			fmt.Printf("%s already exists, skipping test (use -f to force)\n", g)
			return
		}
		rst.push([]reporter{&saveData{g}})
	}
	d := make(chan interface{}, dataChanBufSize)
	defer func() {
		if e := rst.pop(); e != nil && err == nil {
			err = e
		}
	}()
	go node.Do(&test.Run, &exeSource{}, c.Control, d)
	err = rst.tee(d, test, &c.Control)
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
func (r *ReportCommand) run() (err error) {
	var c *Config
	if c, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	err = c.Run.do(r, reporterStack{})
	return
}

// do implements doer
func (*ReportCommand) do(test *Test, rst reporterStack) (err error) {
	g := test.outPath(test.DataFile)
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

// VetCommand loads and checks the CUE config.
type VetCommand struct {
}

// run implements command
func (*VetCommand) run() (err error) {
	_, err = LoadConfig(&load.Config{})
	return
}
