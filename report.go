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

// reportData contains stream and packet data for reports.
type reportData struct {
	streams streams
	packets packets
}

// newReportData returns a new reportData.
func newReportData() *reportData {
	return &reportData{newStreams(), newPackets()}
}

// add adds a report data item from the result stream.
func (r *reportData) add(a interface{}) {
	switch v := a.(type) {
	case node.StreamInfo:
		d := r.streams.data(v.Flow)
		d.Info = v
	case node.StreamIO:
		d := r.streams.data(v.Flow)
		if v.Sent {
			d.Sent = append(d.Sent, v)
		} else {
			d.Rcvd = append(d.Rcvd, v)
		}
	case node.PacketInfo:
		d := r.packets.data(v.Flow)
		d.Info = v
	case node.PacketIO:
		d := r.packets.data(v.Flow)
		if v.Sent {
			d.Sent = append(d.Sent, v)
		} else {
			d.Rcvd = append(d.Rcvd, v)
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (r *reportData) analyze() {
	ss := r.streams.StartTime()
	ps := r.packets.StartTime()
	st := ss
	if st.IsZero() || (!ps.IsZero() && ps.Before(st)) {
		st = ps
	}
	r.streams.synchronize(st)
	r.packets.synchronize(st)
	r.streams.analyze()
	r.packets.analyze()
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
	var t metric.RelativeTime
	if len(s.Sent) == 0 {
		if len(s.Rcvd) == 0 {
			return time.Time{}
		}
		t = s.Rcvd[0].T
	} else if len(s.Rcvd) == 0 {
		t = s.Sent[0].T
	} else {
		if s.Sent[0].T < s.Rcvd[0].T {
			t = s.Sent[0].T
		} else {
			t = s.Rcvd[0].T
		}
	}
	return s.Info.Time(t)
}

// goodput is a single goodput data point.
type goodput struct {
	// T is the time relative to the start of the earliest stream.
	T metric.RelativeTime

	// Goodput is the goodput bitrate.
	Goodput metric.Bitrate
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
func (m *streams) synchronize(start time.Time) {
	for _, r := range *m {
		for i := 0; i < len(r.Sent); i++ {
			io := &r.Sent[i]
			t := io.T.Time(r.Info.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(r.Rcvd); i++ {
			io := &r.Rcvd[i]
			t := io.T.Time(r.Info.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (m *streams) analyze() {
	for _, s := range *m {
		var pr node.StreamIO
		for _, r := range s.Rcvd {
			var g metric.Bitrate
			if pr != (node.StreamIO{}) {
				g = metric.CalcBitrate(r.Total-pr.Total,
					time.Duration(r.T-pr.T))
			}
			s.Goodput = append(s.Goodput, goodput{r.T, g})
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

// owd is a single one-way delay data point.
type owd struct {
	// T is the time relative to the start of the test.
	T metric.RelativeTime

	// Delay is the one-way delay.
	Delay time.Duration
}

// packetData contains the data and calculated stats for a packet flow.
type packetData struct {
	Info node.PacketInfo
	Sent []node.PacketIO
	Rcvd []node.PacketIO
	OWD  []owd
}

// T0 returns the earliest absolute packet time.
func (k *packetData) T0() time.Time {
	var t metric.RelativeTime
	if len(k.Sent) == 0 {
		if len(k.Rcvd) == 0 {
			return time.Time{}
		}
		t = k.Rcvd[0].T
	} else if len(k.Rcvd) == 0 {
		t = k.Sent[0].T
	} else {
		if k.Sent[0].T < k.Rcvd[0].T {
			t = k.Sent[0].T
		} else {
			t = k.Rcvd[0].T
		}
	}
	return k.Info.Time(t)
}

// packets aggregates data for multiple packet flows.
type packets map[node.Flow]*packetData

// newPackets returns a new packets.
func newPackets() packets {
	return packets(make(map[node.Flow]*packetData))
}

// data adds packetData for the given flow if it doesn't already exist.
func (k *packets) data(flow node.Flow) (d *packetData) {
	var ok bool
	if d, ok = (*k)[flow]; ok {
		return
	}
	d = &packetData{}
	(*k)[flow] = d
	return
}

// StartTime returns the earliest absolute start time among the packet flows.
func (k *packets) StartTime() (start time.Time) {
	for _, d := range *k {
		t0 := d.T0()
		if start.IsZero() || t0.Before(start) {
			start = t0
		}
	}
	return
}

// synchronize adjusts the PacketIO RelativeTime values from node-relative to
// test-relative time.
func (k *packets) synchronize(start time.Time) {
	for _, p := range *k {
		for i := 0; i < len(p.Sent); i++ {
			io := &p.Sent[i]
			t := io.T.Time(p.Info.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(p.Rcvd); i++ {
			io := &p.Rcvd[i]
			t := io.T.Time(p.Info.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (k *packets) analyze() {
	for _, p := range *k {
		var s, r node.PacketIO
		for _, s = range p.Sent {
			for _, r = range p.Rcvd {
				if s.Seq == r.Seq {
					d := time.Duration(r.T - s.T)
					p.OWD = append(p.OWD, owd{r.T, d})
				}
			}
		}
	}
}

// byTime returns a slice of packetData, sorted by start time.
func (k *packets) byTime() (d []packetData) {
	for _, p := range *k {
		d = append(d, *p)
	}
	sort.Slice(d, func(i, j int) bool {
		return d[i].T0().Before(d[j].T0())
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
