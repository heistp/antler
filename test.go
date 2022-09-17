// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"os"
	"path"
	"strings"

	"github.com/heistp/antler/node"
)

// Test is an Antler test.
type Test struct {
	// ID uniquely identifies a Test within a CUE package. The ID's key/value
	// pairs may be used to generate output filenames and/or report lists/tables
	// that identify the key Test properties, e.g. bandwidth, rtt, etc.
	ID ID

	// OutputPath is the base path for test output files, relative to the output
	// directory. Paths ending in '/' are a directory, and '/' is appended
	// automatically if the path is a directory. The default is "./".
	OutputPath string

	// Run is the top-level Run instance.
	node.Run
}

// ID represents a compound Test identifier consisting of key/value pairs.
type ID map[string]string

// dataChanBuf is used as the buffer size for data channels.
const dataChanBuf = 64

// do runs the Test and tees the data stream to reporters.
func (t *Test) do(ctrl *node.Control, rst reporterStack) (err error) {
	d := make(chan interface{}, dataChanBuf)
	go node.Do(&t.Run, &exeSource{}, ctrl, d)
	err = t.tee(ctrl, rst, d)
	return
}

// outPath returns the path to an output file with the given suffix. OutputPath
// is first normalized, appending "/" to the path if it refers to a directory.
func (t *Test) outPath(suffix string) string {
	p := t.OutputPath
	var d bool
	if strings.HasSuffix(p, "/") {
		d = true
	} else if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		d = true
		p += "/"
	}
	if d {
		return path.Join(p, suffix)
	}
	return p + "_" + suffix
}

// tee receives data from the given channel, and sends it to each reporter in
// the stack. On the first error, the node is canceled and the error returned.
func (t *Test) tee(ctrl *node.Control, rst reporterStack,
	data chan interface{}) (err error) {
	ec := make(chan error)
	var cc []chan interface{}
	for _, r := range rst.list() {
		c := make(chan interface{}, dataChanBuf)
		cc = append(cc, c)
		r.report(reportIn{t, c, ec})
	}
	n := rst.size()
	dc := data
	for n > 0 || dc != nil {
		select {
		case e := <-ec:
			if e == reportDone {
				n--
				break
			}
			if err == nil {
				err = e
				ctrl.Cancel(e.Error())
			}
		case d, ok := <-dc:
			if !ok {
				dc = nil
				for _, c := range cc {
					close(c)
				}
				break
			}
			for _, c := range cc {
				c <- d
			}
		}
	}
	return
}

/*
// gatherer reads the data and writes it to the appropriate output files.
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

// run gathers all data from the given channel, until closed, and returns the
// first error. On error, Cancel is called on Control.
func (g *gatherer) run(data chan interface{}) error {
	var e error
	if e = g.openData(); e != nil {
		g.handleError(e)
	}
	defer g.closeFiles()
	for r := range data {
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
			fmt.Printf("%s\n", v)
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
			panic(fmt.Sprintf("gather received unknown data type: %T", r))
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

// closeFiles closes all the opened files and returns the first error.
func (g *gatherer) closeFiles() (err error) {
	for n, f := range g.file {
		if e := f.Close(); e != nil && err == nil {
			err = e
		}
		delete(g.file, n)
	}
	if g.data != nil {
		if e := g.data.Close(); e != nil && err == nil {
			err = e
		}
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

// handleError handles an error during run.
func (g *gatherer) handleError(e error) {
	e = fmt.Errorf("error gathering data: %w", e)
	if g.err == nil {
		g.err = e
		g.ctrl.Cancel(e.Error())
	}
	g.appendData(node.Error{time.Now(), node.RootNodeID, "gather", e.Error()})
}
*/
