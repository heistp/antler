// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"github.com/heistp/antler/node"
)

// Test is an Antler test.
type Test struct {
	// Props maps property names to values, which together are used to uniquely
	// identify a Test within a CUE package. These properties may be used to
	// generate output filenames and/or report lists/tables that identify the
	// key Test properties, e.g. bandwidth, rtt, etc.
	Props map[string]interface{}

	// Run is the top-level Run instance.
	node.Run
}

// do runs the Test and saves the results.
func (t *Test) do(ctrl *node.Control, arg doArg) (err error) {
	r := make(chan interface{}, 64)
	go node.Do(&t.Run, &exeSource{}, ctrl, r)
	g := newGatherer(ctrl)
	err = g.run(r, arg)
	return
}

// gatherer reads the results and writes them to the appropriate output files.
type gatherer struct {
	ctrl *node.Control
	file map[string]*os.File
	data *os.File
	enc  *gob.Encoder
	err  error
}

// newGatherer returns a new instance of gatherer.
func newGatherer(ctrl *node.Control) *gatherer {
	return &gatherer{ctrl, make(map[string]*os.File), nil, nil, nil}
}

// run gathers all results from the given channel, until closed, and returns the
// first error. On error, Cancel is called on Control.
func (g *gatherer) run(result chan interface{}, arg doArg) error {
	var e error
	if e = g.openData(); e != nil {
		g.handleError(e)
	}
	defer func() {
		g.closeFiles()
		g.closeData()
	}()
	for r := range result {
		switch v := r.(type) {
		case node.DataPoint:
			if e = g.appendData(v); e != nil {
				g.handleError(e)
			}
		case node.FileData:
			if e = g.appendFile(v); e != nil {
				g.handleError(e)
			}
		case node.LogEntry:
			if arg.Log {
				fmt.Printf("%s\n", v)
			}
			if e = g.appendData(v); e != nil {
				g.handleError(e)
			}
		case node.Error:
			fmt.Fprintf(os.Stderr, "%s\n", v)
			if g.err == nil {
				g.err = v
			}
			if e = g.appendData(v); e != nil {
				g.handleError(e)
			}
		default:
			panic(fmt.Sprintf("gather received unknown result type: %T", r))
		}
	}
	return g.err
}

// appendFile appends FileData to a file, opening the file for write as
// necessary, and retaining already open file handles.
func (g *gatherer) appendFile(fd node.FileData) (err error) {
	var ok bool
	var f *os.File
	if f, ok = g.file[fd.Name]; !ok {
		if f, err = os.Create(fd.Name); err != nil {
			return
		}
		g.file[fd.Name] = f
	}
	_, err = f.Write(fd.Data)
	return
}

// closeFiles closes all the FileData files and returns the first error.
func (g *gatherer) closeFiles() (err error) {
	for n, f := range g.file {
		if e := f.Close(); e != nil && err == nil {
			err = e
		}
		delete(g.file, n)
	}
	return
}

// dataFileName is the fixed name of the data output file.
const dataFileName = "data.gob"

// openData opens the data file for write and creates the gob Encoder.
func (g *gatherer) openData() (err error) {
	if g.data, err = os.Create(dataFileName); err != nil {
		return
	}
	g.enc = gob.NewEncoder(g.data)
	return
}

// appendData append data to the data file, if it was successfully opened.
func (g *gatherer) appendData(v interface{}) error {
	if g.enc == nil {
		return nil
	}
	return g.enc.Encode(v)
}

// closeData closes the data file.
func (g *gatherer) closeData() (err error) {
	if g.data != nil {
		return g.data.Close()
	}
	return nil
}

// handleError handles an error during run.
func (g *gatherer) handleError(e error) {
	e = fmt.Errorf("error gathering results: %w", e)
	if g.err == nil {
		g.err = e
		g.ctrl.Cancel(e.Error())
	}
	g.appendData(node.Error{time.Now(), node.RootNodeID, "gather", e.Error()})
}
