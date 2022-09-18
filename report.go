// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/heistp/antler/node"
)

// A reporter can process data items from the node and take some action, such as
// saving results, generating plots, or emitting logs.
//
// When report is called, the implementation must handle the input according to
// the documentation for reportIn, asynchronously.
//
// The report method may be called during TestRun execution multiple times,
// possibly concurrently, so it must be safe for concurrent use.
//
// If a reporter needs to finalize its work, it may also implement io.Closer.
// If it does, Close will be called after all reports are complete.
type reporter interface {
	report(reportIn)
}

// reportIn is sent to a reporter to do its work. The goroutine that handles the
// reportIn must receive from the data channel until it's closed. It may send
// any errors on errc, and must send reportDone on errc when complete.
//
// The data channel may be closed by the sender at any time to cancel the
// report. Therefore, receivers must be prepared to handle partial input, and
// complete as soon as possible.
type reportIn struct {
	test *Test
	data chan interface{}
	errc chan error
}

// reportDone is sent on the error channel by reporters to indicate they are
// done processing.
var reportDone = errors.New("report done")

// Report is a union of the available reporters.
type Report struct {
	reporters
}

// reporters is a union of the available reporters.
type reporters struct {
	EmitLog   *EmitLog
	SaveFiles *SaveFiles
}

// reporter returns the only non-nil reporter implementation.
func (r *reporters) reporter() reporter {
	switch {
	case r.EmitLog != nil:
		return r.EmitLog
	case r.SaveFiles != nil:
		return r.SaveFiles
	}
	return nil
}

// reports is a slice of Report's, with some convenience methods.
type reports []Report

// reporters returns a slice of reporters for each Report.
func (s reports) reporters() (reps []reporter) {
	for _, r := range s {
		reps = append(reps, r.reporter())
	}
	return
}

// EmitLog is a reporter that emits LogEntry's to files and/or stdout.
type EmitLog struct {
	// To lists the destinations to send output to. "-" sends output to stdout,
	// and everything else sends output to the named file. If To is empty,
	// output is emitted to stdout.
	To []string
}

// report implements reporter
func (l *EmitLog) report(in reportIn) {
	go func() {
		var e error
		var ff []*os.File
		defer func() {
			if e != nil {
				in.errc <- e
			}
			for _, f := range ff {
				f.Close()
			}
			for range in.data {
			}
			in.errc <- reportDone
		}()
		ww := []io.Writer{os.Stdout}
		if len(l.To) > 0 {
			ww = ww[:0]
			for _, s := range l.To {
				if s == "-" {
					ww = append(ww, os.Stdout)
					continue
				}
				n := in.test.outPath(s)
				var f *os.File
				if f, e = os.Create(n); e != nil {
					return
				}
				ww = append(ww, f)
				ff = append(ff, f)
			}
		}
		for d := range in.data {
			switch v := d.(type) {
			case node.LogEntry, node.Error:
				for _, w := range ww {
					if _, e = fmt.Fprintln(w, v); e != nil {
						return
					}
				}
			}
		}
	}()
	return
}

// SaveFiles is a reporter that saves FileData.
type SaveFiles struct {
}

// report implements reporter
func (s *SaveFiles) report(in reportIn) {
	go func() {
		m := make(map[string]*os.File)
		var e error
		defer func() {
			if e != nil {
				in.errc <- e
			}
			for n, f := range m {
				f.Close()
				delete(m, n)
			}
			for range in.data {
			}
			in.errc <- reportDone
		}()
		for d := range in.data {
			var fd node.FileData
			var ok bool
			if fd, ok = d.(node.FileData); !ok {
				continue
			}
			var f *os.File
			if f, ok = m[fd.Name]; !ok {
				n := in.test.outPath(fd.Name)
				if f, e = os.Create(n); e != nil {
					return
				}
				m[fd.Name] = f
			}
			if _, e = f.Write(fd.Data); e != nil {
				return
			}
		}
	}()
	return
}

// saveData is an reporter that saves all data using gob to the named file.
type saveData struct {
	name string
}

// report implements reporter
func (s *saveData) report(in reportIn) {
	go func() {
		var e error
		defer func() {
			if e != nil {
				in.errc <- e
			}
			for range in.data {
			}
			in.errc <- reportDone
		}()
		var f *os.File
		if f, e = os.Create(s.name); e != nil {
			return
		}
		defer f.Close()
		c := gob.NewEncoder(f)
		for d := range in.data {
			if e = c.Encode(&d); e != nil {
				return
			}
		}
	}()
	return
}

// reporterStack is a stack of reporters used when running a TestRun hierarchy.
type reporterStack [][]reporter

// push adds a slice of reporters to the stack.
func (s *reporterStack) push(r []reporter) {
	*s = append(*s, r)
}

// pop pops a slice of reporters from the stack, runs Close on each if it
// implements io.Closer, and returns the first error.
func (s *reporterStack) pop() (err error) {
	rr := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	for _, r := range rr {
		if c, ok := r.(io.Closer); ok {
			if e := c.Close(); e != nil && err == nil {
				err = e
			}
		}
	}
	return
}

// list returns a flat list of reporters in the stack.
func (s *reporterStack) list() (l []reporter) {
	for _, r := range *s {
		l = append(l, r...)
	}
	return
}

// size returns the numbers of reporters in the stack.
func (s *reporterStack) size() (sz int) {
	for _, r := range *s {
		sz += len(r)
	}
	return
}
