// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"

	"github.com/heistp/antler/node"
)

// A reporter can process data items from the node for a single Test. It is run
// as a stage in a pipeline, where data items are received on the in channel,
// and sent on the out channel. Reporters may consume, emit or forward data
// items. Reporters should forward any unrecognized or unhandled items.
//
// Reporters may return at any time, with or without an error. Any remaining
// data on their in channel will be forwarded to the out channel.
//
// Reporters may use the given Context to react to cancellation signals, and if
// canceled, should return the error from context.Cause(ctx). Reporters may also
// ignore the Context. In any case, they should expect that partial input data
// is possible, in which case an error should be returned.
//
// Reporters may read or write results using the given 'rwer'.
//
// Both in and out are always non-nil channels.
//
// If a reporter is the first stage in a pipeline with no input, the in channel
// will be closed immediately with no input data.
//
// If a reporter is the last stage in a pipeline, it may send nothing to out.
// For configuration flexibility, most reports should forward data to out as
// usual, unless they are certain to be the last stage in the pipeline.
//
// Reporters should be concurrent safe.
type reporter interface {
	report(ctx context.Context, rw rwer, in <-chan any, out chan<- any) error
}

// Report represents a list of reporters.
type Report []reporters

// report returns an equivalent report instance.
func (r Report) report() (t report) {
	for _, p := range r {
		t = append(t, p.reporter())
	}
	return
}

// reporters is a union of the available reporters.
type reporters struct {
	Analyze          *Analyze
	EmitLog          *EmitLog
	EmitSysInfo      *EmitSysInfo
	ChartsFCT        *ChartsFCT
	ChartsTimeSeries *ChartsTimeSeries
	SaveFiles        *SaveFiles
	Encode           *Encode
}

// reporter returns the only non-nil reporter implementation.
func (r *reporters) reporter() reporter {
	switch {
	case r.Analyze != nil:
		return r.Analyze
	case r.EmitLog != nil:
		return r.EmitLog
	case r.EmitSysInfo != nil:
		return r.EmitSysInfo
	case r.ChartsFCT != nil:
		return r.ChartsFCT
	case r.ChartsTimeSeries != nil:
		return r.ChartsTimeSeries
	case r.SaveFiles != nil:
		return r.SaveFiles
	case r.Encode != nil:
		return r.Encode
	default:
		panic("no reporter set in reporters union")
	}
}

// report is a Report list with the reporters unions resolved to implementations
// of the reporter interface.
type report []reporter

// add appends another report to this one.
func (r report) add(other report) report {
	return append(r, other...)
}

// pipeline confines goroutines to run the reporters in a pipeline. See the
// reporter interface documentation for its contract.
//
// If in is not nil, the caller is expected to send to in and close it when
// done. If out is not nil, the caller is expected to receive all items from it
// until closed.
//
// If the report has no reporters, a nopReport is added so the pipeline still
// functions per the contract.
//
// The returned error channel receives any errors that occur, and is closed when
// the pipeline is done, meaning all of its stages are done.
func (r report) pipeline(ctx context.Context, rw rwer, in chan any,
	out chan any) <-chan error {
	if len(r) == 0 {
		r = append(r, nopReport{})
	}
	err := make(chan error)
	ec := make(chan error)
	cc := make([]chan any, len(r)+1)
	if cc[0] = in; cc[0] == nil {
		cc[0] = make(chan any)
		close(cc[0])
	}
	for i := 1; i < len(r); i++ {
		cc[i] = make(chan any, dataChanBufLen)
	}
	var g int
	if cc[len(r)] = out; cc[len(r)] == nil {
		cc[len(r)] = make(chan any, dataChanBufLen)
		g++
		go func() {
			defer func() {
				ec <- nil
			}()
			for range cc[len(r)] {
			}
		}()
	}
	for i, t := range r {
		g++
		go func(t reporter, in <-chan any, out chan<- any) {
			var e error
			defer func() {
				for a := range in {
					out <- a
				}
				close(out)
				if p := recover(); p != nil {
					e = fmt.Errorf("pipeline panic in %T: %s\n%s",
						t, p, string(debug.Stack()))
				}
				ec <- e
			}()
			e = t.report(ctx, rw, in, out)
		}(t, cc[i], cc[i+1])
	}
	go func() {
		for i := 0; i < g; i++ {
			if e := <-ec; e != nil {
				err <- e
			}
		}
		close(err)
	}()
	return err
}

// tee confines goroutines to pipeline this report to concurrent pipelines for
// each of the given reports. The output for each 'to' report is nil. The
// returned error channel receives any errors that occur, and is closed when
// the tee is done, meaning each of the pipelines is done.
func (r report) tee(ctx context.Context, rw rwer, in chan any,
	to ...report) <-chan error {
	var c []chan any
	for range to {
		c = append(c, make(chan any, dataChanBufLen))
	}
	t := tee(c...)
	var ec []<-chan error
	ec = append(ec, r.pipeline(ctx, rw, in, t))
	for i, p := range to {
		ec = append(ec, p.pipeline(ctx, rw, c[i], nil))
	}
	oc := make(chan error)
	go func() {
		for e := range mergeErr(ec...) {
			oc <- e
		}
		close(oc)
	}()
	return oc
}

// nopReport is a reporter for internal use that does nothing.
type nopReport struct {
}

// report implements reporter
func (nopReport) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	return
}

// SaveFiles is a reporter that saves FileData. If Consume is true, FileData
// items are not forwarded to the out channel.
type SaveFiles struct {
	Consume bool
}

// report implements reporter
func (s *SaveFiles) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	m := make(map[string]io.WriteCloser)
	defer func() {
		for n, w := range m {
			if e := w.Close(); e != nil && err == nil {
				err = e
			}
			delete(m, n)
		}
	}()
	for d := range in {
		var fd node.FileData
		var ok bool
		if fd, ok = d.(node.FileData); !ok {
			out <- d
			continue
		}
		var w io.WriteCloser
		if w, ok = m[fd.Name]; !ok {
			w = rw.Writer(fd.Name)
			m[fd.Name] = w
			out <- FileRef{fd.Name}
		}
		if _, err = w.Write(fd.Data); err != nil {
			return
		}
		if !s.Consume {
			out <- d
		}
	}
	return
}

// FileRef is sent as a data item by SaveFiles to record the presence of a file
// with the specified Name, even after its FileData items may have been
// consumed.
type FileRef struct {
	Name string
}

// init registers FileRef with the gob encoder.
func init() {
	gob.Register(FileRef{})
}

// Encode is a reporter that encodes files referenced by FileRefs.
type Encode struct {
	File        []string // list of glob patterns of files to encode
	Extension   string   // extension for newly encoded files (e.g. ".gz")
	ReEncode    bool     // if true, allow re-encoding of file
	Destructive bool     // if true, delete originals upon success
}

// report implements reporter
func (c *Encode) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	for d := range in {
		if f, ok := d.(FileRef); ok {
			var m bool
			if m, err = c.match(f.Name); err != nil {
				return
			}
			if !m {
				continue
			}
			if err = c.encode(f.Name, rw); err != nil {
				return
			}
		}
		out <- d
	}
	return
}

// match reports whether name matches any of the patterns in the File field.
func (c *Encode) match(name string) (matched bool, err error) {
	for _, p := range c.File {
		if matched, err = filepath.Match(p, name); matched || err != nil {
			return
		}
	}
	return
}

// encode encodes, re-encodes or decodes the named file.
func (c *Encode) encode(name string, rw rwer) (err error) {
	var r *ResultReader
	if r, err = rw.Reader(name); err != nil {
		return
	}
	defer func() {
		if e := r.Close(); e != nil && err == nil {
			err = e
		}
	}()
	var w *ResultWriter
	w = rw.Writer(name + c.Extension)
	defer func() {
		if e := w.Close(); e != nil && err == nil {
			err = e
		}
	}()
	if !c.ReEncode && r.Codec.Equal(w.Codec) {
		return
	}
	_, err = io.Copy(w, r)
	if err == nil && c.Destructive && r.Path != w.Path {
		err = rw.Remove(r.Path)
	}
	return
}

// readData is an internal reporter that reads data items from the ReadCloser
// that reads a gob file, and sends them to the out channel. readData expects to
// be the first stage in a pipeline, so any input is first discarded.
//
// If a decoding error occurs, the error is returned immediately.
//
// If the Context is canceled, sending is stopped and the error from
// context.Cause() is returned.
type readData struct {
	io.ReadCloser
}

// report implements reporter
func (r readData) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	defer r.Close()
	for range in {
	}
	c := gob.NewDecoder(r)
	for {
		var a any
		if err = c.Decode(&a); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		out <- a
		select {
		case <-ctx.Done():
			err = context.Cause(ctx)
			return
		default:
		}
	}
}

// writeData is a WriteCloser and internal reporter that writes data using gob.
// writeData expects to be the final stage in a pipeline, so all data is
// consumed.
//
// If an encoding error occurs, the error is returned immediately.
//
// If the data includes any errors, the first error is returned after reading
// and saving all the data.
type writeData struct {
	io.WriteCloser
}

// report implements reporter
func (w writeData) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	defer func() {
		if e := w.Close(); e != nil && err == nil {
			err = e
		}
	}()
	c := gob.NewEncoder(w)
	for d := range in {
		if e := c.Encode(&d); e != nil {
			err = e
			return
		}
		if e, ok := d.(error); ok && err == nil {
			err = e
		}
	}
	return
}

// rangeData is an internal reporter that sends data from its slice to out.
// rangeData expects to be the first stage in a pipeline, so "in" is first
// discarded.
//
// If the Context is canceled, sending is stopped and the error from
// context.Cause() is returned.
type rangeData []any

// report implements reporter
func (r rangeData) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	for range in {
	}
	for _, a := range r {
		out <- a
		select {
		case <-ctx.Done():
			err = context.Cause(ctx)
			return
		default:
		}
	}
	return
}

// appendData is an internal reporter that buffers data in its slice. appendData
// expects to be the final stage in a pipeline, so all data is consumed.
//
// If the data includes any errors, the first error is returned after reading
// and buffering all the data.
type appendData []any

// report implements reporter
func (a *appendData) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) error {
	var f error
	for d := range in {
		*a = append(*a, d)
		if e, ok := d.(error); ok && f == nil {
			f = e
		}
	}
	return f
}

// A multiReporter can process data items for multiple Tests. It receives its
// input from the final stage of the Test.After pipeline.
//
// MultiReporters may use the given Context to react to cancellation signals,
// and if canceled, should return the error from context.Cause(ctx).
// MultiReporters may also ignore the Context. In any case, they should expect
// that partial input data is possible, in which case an error should be
// returned if it is known that this would affect the output.
//
// MultiReporters should be able to handle data from multiple input streams
// concurrently.
//
// MultiReporters may return with or without an error, however, they must not do
// so until draining the in channel from any testData channels they have read,
// and until any goroutines that were started have completed.
//
// TODO update the above contract after implementation
type multiReporter interface {
	report(ctx context.Context, work resultRW, data <-chan testData) error
}

// A testData contains a Test and its data stream for a multiReporter.
type testData struct {
	test *Test
	in   <-chan any
}

// multiReporters is a union of the available multiReporters.
type multiReporters struct {
	Index *Index
}

// multiReporter returns the only non-nil multiReporter implementation.
func (m *multiReporters) multiReporter() multiReporter {
	switch {
	case m.Index != nil:
		return m.Index
	default:
		panic("no multiReporter set in multiReporters union")
	}
}

// MultiReport represents a list of multiReporters.
type MultiReports []multiReporters
