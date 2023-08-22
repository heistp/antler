// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
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
// When report is called, the implementation must handle the input
// asynchronously, according to the documentation for reportIn.
//
// The report method may be called during TestRun execution multiple times,
// possibly concurrently, so it must be safe for concurrent use.
//
// If a reporter needs to finalize its work, it may also implement io.Closer.
// If it does, Close will be called after all reports are complete.
type reporter interface {
	report(reportIn)
}

// reportIn is sent to a reporter to do its work.
//
// The goroutine that handles the reportIn must receive from the data channel
// until it's closed. It may send any errors on errc, and must send reportDone
// on errc when complete.
//
// The data channel may be closed by the sender at any time to cancel the
// report. Therefore, receivers must be prepared to handle partial input, and
// complete as soon as possible.
//
// The embedded writerer provides an Open method for writing result data.
type reportIn struct {
	writerer
	data chan any
	errc chan error
}

// writerer wraps the Writer method, to return a WriteCloser for writing test
// output. The name parameter identifies the result data according to the
// underlying implementation, and is typically a filename, or filename suffix.
type writerer interface {
	Writer(name string, overwrite bool) (io.WriteCloser, error)
}

// reportDone is sent on the error channel by reporters to indicate they are
// done processing.
var reportDone = errors.New("report done")

// Report represents the report configuration.
type Report struct {
	reporters
}

// reporters is a union of the available reporters.
type reporters struct {
	EmitLog          *EmitLog
	ChartsFCT        *ChartsFCT
	ChartsTimeSeries *ChartsTimeSeries
	SaveFiles        *SaveFiles
}

// reporter returns the only non-nil reporter implementation.
func (r *reporters) reporter() reporter {
	switch {
	case r.EmitLog != nil:
		return r.EmitLog
	case r.ChartsFCT != nil:
		return r.ChartsFCT
	case r.ChartsTimeSeries != nil:
		return r.ChartsTimeSeries
	case r.SaveFiles != nil:
		return r.SaveFiles
	default:
		panic("no reporter set in reporters union")
	}
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

// reporterStack is a stack of reporters used when running a TestRun hierarchy.
type reporterStack [][]reporter

// push adds a slice of reporters to the stack.
func (s *reporterStack) push(r []reporter) {
	*s = append(*s, r)
}

// pop pops a slice of reporters from the stack.
func (s *reporterStack) pop() {
	*s = (*s)[:len(*s)-1]
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

// tee receives data from the given channel, and sends it to each reporter in
// the stack. On the first error, the context is canceled. After data is read
// in full, the first error, if any, is returned.
func (s *reporterStack) tee(cancel context.CancelCauseFunc, data chan any,
	wr writerer) (err error) {
	ec := make(chan error)
	var cc []chan any
	for _, r := range s.list() {
		c := make(chan any, dataChanBufSize)
		cc = append(cc, c)
		r.report(reportIn{wr, c, ec})
	}
	n := s.size()
	a := newAnalysis()
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
				cancel(e)
			}
		case d, ok := <-dc:
			if !ok {
				dc = nil
				a.analyze()
				for _, c := range cc {
					c <- a
					close(c)
				}
				break
			}
			a.add(d)
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

// EmitLog is a reporter that emits LogEntry's to files and/or stdout.
type EmitLog struct {
	// To lists the destinations to send output to. "-" sends output to stdout,
	// and everything else sends output to the named file. If To is empty,
	// output is emitted to stdout.
	To []string
}

// report implements reporter
func (l *EmitLog) report(in reportIn) {
	var f simpleReportFunc = l.reportOne
	f.report(in)
}

// reportOne runs one EmitLog reporter.
func (l *EmitLog) reportOne(in reportIn) (err error) {
	ww := []io.WriteCloser{os.Stdout}
	defer func() {
		for _, w := range ww {
			if w != nil && w != os.Stdout {
				w.Close()
			}
		}
	}()
	if len(l.To) > 0 {
		ww = ww[:0]
		for _, s := range l.To {
			if s == "-" {
				ww = append(ww, os.Stdout)
				continue
			}
			var w io.WriteCloser
			if w, err = in.Writer(s, true); err != nil {
				return
			}
			ww = append(ww, w)
		}
	}
	for d := range in.data {
		switch v := d.(type) {
		case node.LogEntry, node.Error:
			for _, w := range ww {
				if _, err = fmt.Fprintln(w, v); err != nil {
					return
				}
			}
		}
	}
	return
}

// SaveFiles is a reporter that saves FileData.
type SaveFiles struct {
}

// report implements reporter
func (s *SaveFiles) report(in reportIn) {
	var f simpleReportFunc = s.reportOne
	f.report(in)
}

// reportOne runs one SaveFiles reporter.
func (s *SaveFiles) reportOne(in reportIn) (err error) {
	m := make(map[string]io.WriteCloser)
	defer func() {
		for n, w := range m {
			w.Close()
			delete(m, n)
		}
	}()
	for d := range in.data {
		var fd node.FileData
		var ok bool
		if fd, ok = d.(node.FileData); !ok {
			continue
		}
		var w io.WriteCloser
		if w, ok = m[fd.Name]; !ok {
			if w, err = in.Writer(fd.Name, true); err != nil {
				return
			}
			m[fd.Name] = w
		}
		if _, err = w.Write(fd.Data); err != nil {
			return
		}
	}
	return
}

// saveData is a WriteCloser that implements reporter to save result data using
// gob.
type saveData struct {
	io.WriteCloser
}

// report implements reporter
func (s saveData) report(in reportIn) {
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
		defer s.Close()
		c := gob.NewEncoder(s)
		for d := range in.data {
			switch d.(type) {
			case node.FileData, analysis:
				continue
			}
			if e = c.Encode(&d); e != nil {
				return
			}
		}
	}()
	return
}

// simpleReportFunc is a function that may be called in a separate goroutine for
// each Test, independently and concurrently. It implements reporter to run a
// goroutine for each call, and provide boilerplate cleanup.
type simpleReportFunc func(reportIn) error

// report implements reporter
func (r simpleReportFunc) report(in reportIn) {
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
		e = r(in)
	}()
}
