// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"sort"
	"time"

	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"

	"github.com/heistp/antler/node"
	"github.com/heistp/antler/node/metric"
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

// Report represents the report configuration.
type Report struct {
	reporters
}

// reporters is a union of the available reporters.
type reporters struct {
	EmitLog          *EmitLog
	ExecuteTemplate  *ExecuteTemplate
	ChartsTimeSeries *ChartsTimeSeries
	SaveFiles        *SaveFiles
}

// reporter returns the only non-nil reporter implementation.
func (r *reporters) reporter() reporter {
	switch {
	case r.EmitLog != nil:
		return r.EmitLog
	case r.ExecuteTemplate != nil:
		return r.ExecuteTemplate
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
	var ff []*os.File
	defer func() {
		for _, f := range ff {
			f.Close()
		}
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
			if f, err = os.Create(n); err != nil {
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
	m := make(map[string]*os.File)
	defer func() {
		for n, f := range m {
			f.Close()
			delete(m, n)
		}
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
			if f, err = os.Create(n); err != nil {
				return
			}
			m[fd.Name] = f
		}
		if _, err = f.Write(fd.Data); err != nil {
			return
		}
	}
	return
}

// ExecuteTemplate is a reporter that executes a Go template and saves the
// results to a file.
type ExecuteTemplate struct {
	// Name is the name of the template.
	Name string

	// From is the names of files to parse the template from
	// (template.ParseFiles).
	From []string

	// Text is the body of the template, to be parsed by Template.Parse.
	Text string

	// To is the name of a file to execute the template to, or "-" for stdout.
	To string
}

// report implements reporter
func (x *ExecuteTemplate) report(in reportIn) {
	var f simpleReportFunc = x.reportOne
	f.report(in)
}

// reportOne runs one ExecuteTemplate report.
func (x *ExecuteTemplate) reportOne(in reportIn) (err error) {
	type templateData struct {
		Test *Test
		Data chan interface{}
	}
	var w io.WriteCloser
	defer func() {
		if w != nil && w != os.Stdout {
			w.Close()
		}
	}()
	var t *template.Template
	if x.Text != "" {
		if t, err = template.New(x.Name).Parse(x.Text); err != nil {
			return
		}
	} else {
		var f []string
		for _, n := range x.From {
			f = append(f, in.test.outPath(n))
		}
		if t, err = template.ParseFiles(f...); err != nil {
			return
		}
	}
	w = os.Stdout
	if x.To != "-" {
		if w, err = os.Create(x.To); err != nil {
			return
		}
	}
	err = t.Execute(w, templateData{in.test, in.data})
	return
}

// streamData contains the data and calculated stats for a stream.
type streamData struct {
	Info    node.StreamInfo
	Sent    []node.StreamIO
	Rcvd    []node.StreamIO
	Goodput []goodput
}

// T0 returns the earliest absolute time from Sent or Rcvd.
func (s *streamData) T0() time.Time {
	if s.Sent[0].T < s.Rcvd[0].T {
		return s.Info.Time(s.Sent[0].T)
	}
	return s.Info.Time(s.Rcvd[0].T)
}

// goodput is a single goodput data point.
type goodput struct {
	// T is the time offset relative to the start of the earliest stream.
	T metric.RelativeTime

	// Goodput is the goodput bitrate.
	Goodput metric.Bitrate

	// First is true for the first goodput point.
	First bool

	// Last is true for the last goodput point.
	Last bool
}

// MbpsRow returns a row of values containing T in the first column, and Goodput
// in Mbps in the stream'th column, with other columns up to streams+1
// containing nil.
func (g goodput) MbpsRow(stream, streams int, first, last bool) (
	a []interface{}) {
	a = append(a, g.T.Duration().Seconds())
	for i := 0; i < streams; i++ {
		if i != stream {
			a = append(a, nil)
			continue
		}
		a = append(a, g.Goodput.Mbps())
	}
	if first {
		a = append(a, "point { visible: true; size: 4; shape-type: diamond; }")
	} else if last {
		a = append(a, "point { visible: true; size: 4; shape-type: circle; }")
	} else {
		a = append(a, nil)
	}
	return
}

// streams aggregates data for multiple streams.
type streams map[node.Flow]*streamData

// newStreams returns a new streams.
func newStreams() streams {
	return streams(make(map[node.Flow]*streamData))
}

// data adds streamData for the given flow if it doesn't already exist.
func (m *streams) data(flow node.Flow) (s *streamData) {
	var ok bool
	if s, ok = (*m)[flow]; ok {
		return
	}
	s = &streamData{}
	(*m)[flow] = s
	return
}

// StartTime returns the earliest absolute start time among the streams.
func (m *streams) StartTime() (start time.Time) {
	for _, s := range *m {
		t0 := s.T0()
		if start.IsZero() || t0.Before(start) {
			start = t0
		}
	}
	return
}

// synchronize adjusts the StreamIO RelativeTime values from node-relative to
// test-relative time.
func (m *streams) synchronize() {
	st := m.StartTime()
	for _, r := range *m {
		for i := 0; i < len(r.Sent); i++ {
			io := &r.Sent[i]
			t := io.T.Time(r.Info.Tinit)
			io.T = metric.RelativeTime(t.Sub(st))
		}
		for i := 0; i < len(r.Rcvd); i++ {
			io := &r.Rcvd[i]
			t := io.T.Time(r.Info.Tinit)
			io.T = metric.RelativeTime(t.Sub(st))
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (m *streams) analyze() {
	m.synchronize()
	for _, s := range *m {
		var pr node.StreamIO
		for i, r := range s.Rcvd {
			var g metric.Bitrate
			if pr != (node.StreamIO{}) {
				g = metric.CalcBitrate(r.Total-pr.Total,
					time.Duration(r.T-pr.T))
			}
			s.Goodput = append(s.Goodput,
				goodput{r.T, g, i == 0, i == len(s.Rcvd)-1})
			pr = r
		}
	}
}

// byTime returns a slice of streamData, sorted by start time.
func (m *streams) byTime() (s []streamData) {
	for _, d := range *m {
		s = append(s, *d)
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].T0().Before(s[j].T0())
	})
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
			if _, ok := d.(node.FileData); ok {
				continue
			}
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

// tee receives data from the given channel, and sends it to each reporter in
// the stack. On the first error, the node is canceled if the Control is not
// nil. After data is read in full, the first error, if any, is returned.
func (s *reporterStack) tee(data chan interface{}, test *Test,
	ctrl *node.Control) (err error) {
	ec := make(chan error)
	var cc []chan interface{}
	for _, r := range s.list() {
		c := make(chan interface{}, dataChanBufSize)
		cc = append(cc, c)
		r.report(reportIn{test, c, ec})
	}
	n := s.size()
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
				if ctrl != nil {
					ctrl.Cancel(e.Error())
				}
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
