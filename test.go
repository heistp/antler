// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"encoding/gob"
	"errors"
	"io"
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

// dataChanBufSize is used as the buffer size for data channels.
const dataChanBufSize = 64

// do runs the Test and tees the data stream to reporters.
func (t *Test) do(ctrl *node.Control, rst reporterStack) (err error) {
	d := make(chan interface{}, dataChanBufSize)
	g := t.outPath("data.gob")
	var f *os.File
	if f, err = os.Open(g); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return
		}
		rst.push([]reporter{&saveData{g}})
		defer func() {
			if e := rst.pop(); e != nil && err == nil {
				err = e
			}
		}()
		go node.Do(&t.Run, &exeSource{}, ctrl, d)
	} else {
		go decodeData(f, d)
	}
	err = t.tee(ctrl, rst, d)
	return
}

// decodeData decodes all gob data items from the given reader and writes them
// to the given data channel. The data channel is closed when complete.
func decodeData(reader io.Reader, data chan interface{}) {
	var e error
	defer func() {
		if e != nil {
			data <- e
		}
		defer close(data)
	}()
	d := gob.NewDecoder(reader)
	var a interface{}
	for {
		if e = d.Decode(&a); e != nil {
			if e == io.EOF {
				e = nil
			}
			return
		}
		data <- a
	}
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
		c := make(chan interface{}, dataChanBufSize)
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
			if e, ok := d.(error); ok && err == nil {
				err = e
			}
			for _, c := range cc {
				c <- d
			}
		}
	}
	return
}
