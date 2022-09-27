// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	_ "embed"
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
	EmitLog         *EmitLog
	ExecuteTemplate *ExecuteTemplate
	GTimeSeries     *GTimeSeries
	SaveFiles       *SaveFiles
}

// reporter returns the only non-nil reporter implementation.
func (r *reporters) reporter() reporter {
	switch {
	case r.EmitLog != nil:
		return r.EmitLog
	case r.ExecuteTemplate != nil:
		return r.ExecuteTemplate
	case r.GTimeSeries != nil:
		return r.GTimeSeries
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

// goodput is a single goodput data point.
type goodput struct {
	// T is the time offset, in seconds, relative to the start of the earliest
	// stream.
	T metric.Duration

	// Goodput
	Goodput metric.Bitrate
}

// stream contains the samples, info and calculated stats for a stream.
type stream struct {
	Info    node.StreamInfo
	Sample  []node.StreamSample
	Goodput []goodput
}

// streams aggregates stream data for multiple series.
type streams map[node.Series]*stream

// newStreams returns a new streams.
func newStreams() streams {
	return streams(make(map[node.Series]*stream))
}

// addInfo adds a new stream with the given info. An error is returned if info
// for the stream was already added.
func (m *streams) addInfo(info node.StreamInfo) (err error) {
	if _, ok := (*m)[info.Series]; ok {
		err = fmt.Errorf("duplicate SeriesInfo for '%s'", info.Series)
		return
	}
	(*m)[info.Series] = &stream{info, nil, nil}
	return
}

// addSample adds a StreamSample. An error is returned if the stream was not
// already added with addInfo.
func (mm *streams) addSample(smp node.StreamSample) (err error) {
	var s *stream
	var ok bool
	if s, ok = (*mm)[smp.Series]; !ok {
		err = fmt.Errorf("sample without SeriesInfo for '%s'", smp.Series)
		return
	}
	s.Sample = append(s.Sample, smp)
	return
}

// T0 returns the earliest T0 (start time) among the streams.
func (m *streams) T0() (t0 time.Time) {
	for _, s := range *m {
		if t0.IsZero() || s.Info.T0.Before(t0) {
			t0 = s.Info.T0
		}
	}
	return
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (m *streams) analyze() {
	t0 := m.T0()
	for _, s := range *m {
		o := s.Info.T0.Sub(t0)
		var p *node.StreamSample
		for _, ss := range s.Sample {
			t := metric.Duration(o + ss.T)
			if p == nil {
				s.Goodput = append(s.Goodput, goodput{t, 0})
			} else {
				g := metric.CalcBitrate(ss.Total-p.Total, ss.T-p.T)
				s.Goodput = append(s.Goodput, goodput{t, g})
			}
			p = &ss
		}
	}
}

// gTimeSeriesTemplate is the template for the GTimeSeries reporter.
//
//go:embed gtimeseries.tmpl
var gTimeSeriesTemplate string

// GTimeSeries is a reporter that makes time series plots using Google Charts.
type GTimeSeries struct {
	// To is the name of a file to execute the template to, or "-" for stdout.
	To string
}

// report implements reporter
func (g *GTimeSeries) report(in reportIn) {
	var f simpleReportFunc = g.reportOne
	f.report(in)
}

// report runs one time series report.
func (g *GTimeSeries) reportOne(in reportIn) (err error) {
	type templateData struct {
		Data1 [][]interface{}
		Data2 [][]interface{}
	}
	var w io.WriteCloser
	defer func() {
		if w != nil && w != os.Stdout {
			w.Close()
		}
	}()
	t := template.New("GTimeSeries")
	if t, err = t.Parse(gTimeSeriesTemplate); err != nil {
		return
	}
	s := newStreams()
	for a := range in.data {
		switch v := a.(type) {
		case node.StreamInfo:
			if err = s.addInfo(v); err != nil {
				return
			}
		case node.StreamSample:
			if err = s.addSample(v); err != nil {
				return
			}
		}
	}
	s.analyze()
	w = os.Stdout
	if g.To != "-" {
		if w, err = os.Create(g.To); err != nil {
			return
		}
	}
	d := templateData{[][]interface{}{
		{"", "A"},
		{1, 5},
		{1.5, 20},
		{2, 10},
	}, [][]interface{}{
		{"", "B"},
		{1.5, 5},
		{2, 20},
		{3, 10},
	}}
	err = t.Execute(w, d)
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
