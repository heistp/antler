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

// pipeline confines goroutines to run the reporters in a pipeline.
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
func (r report) pipeline(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) <-chan error {
	if len(r) == 0 {
		r = append(r, nopReport{})
	}
	var ecc errChans
	cc := make([]chan any, len(r)-1)
	// set input channel, or make a closed input channel if nil
	var pin <-chan any
	if pin = in; pin == nil {
		i := make(chan any)
		close(i)
		pin = i
	}
	// make intermediary channels
	for i := 0; i < len(cc); i++ {
		cc[i] = make(chan any, dataChanBufLen)
	}
	// set output channel, or make a drained output channel if nil
	var pout chan<- any
	if pout = out; pout == nil {
		o := make(chan any, dataChanBufLen)
		pout = o
		ec := ecc.make()
		go func(ec chan error) {
			defer close(ec)
			for range o {
			}
		}(ec)
	}
	// start goroutines for each stage
	for x, t := range r {
		i := pin
		if x > 0 {
			i = cc[x-1]
		}
		o := pout
		if x < len(r)-1 {
			o = cc[x]
		}
		ec := ecc.make()
		go func(t reporter, in <-chan any, out chan<- any, ec chan error) {
			defer func() {
				for a := range in {
					out <- a
				}
				close(out)
				if p := recover(); p != nil {
					ec <- fmt.Errorf("pipeline panic in %T: %s\n%s",
						t, p, string(debug.Stack()))
				}
				close(ec)
			}()
			if e := t.report(ctx, rw, in, out); e != nil {
				ec <- e
			}
		}(t, i, o, ec)
	}
	return ecc.merge()
}

// tee confines goroutines to pipeline this report to concurrent pipelines for
// each of the given reports. The output for each 'to' report is nil. The
// returned error channel receives any errors that occur, and is closed when
// the tee is done, meaning each of the pipelines is done.
func (r report) tee(ctx context.Context, rw rwer, in <-chan any,
	to ...report) <-chan error {
	var ic []chan any
	for range to {
		ic = append(ic, make(chan any, dataChanBufLen))
	}
	oc := make(chan any, dataChanBufLen)
	go func() {
		for a := range oc {
			for _, o := range ic {
				o <- a
			}
		}
		for _, o := range ic {
			close(o)
		}
	}()
	var ec errChans
	ec.add(r.pipeline(ctx, rw, in, oc))
	for i, p := range to {
		ec.add(p.pipeline(ctx, rw, ic[i], nil))
	}
	return ec.merge()
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
// input from the final stage of the Test.After pipeline. Implementations should
// read data from the in channel, and may return an error, or nil, at any time.
// They must not close the in channel.
//
// MultiReporters may use the given Context to react to cancellation signals,
// and if canceled, should return the error from context.Cause(ctx).
// MultiReporters may also ignore the Context. In any case, they should expect
// that partial input data is possible, in which case an error should be
// returned if it is known that this would affect the output.
//
// MultiReporters must be concurrent safe, and should be able to concurrently
// process multiple report calls in parallel.
type multiReporter interface {
	report(ctx context.Context, work resultRW, test *Test, in <-chan any) error
}

// A multiStarter can be implemented by a multiReporter to do some
// initialization before the report method is called.
type multiStarter interface {
	start(work resultRW) error
}

// A multiStopper can be implemented by a multiReporter to perform some final
// work or cleanup after the report method has been called.
type multiStopper interface {
	stop(work resultRW) error
}

// MultiReport represents the MultiReport configuration from CUE.
type MultiReport struct {
	ID TestID
	multiReporters
}

// wants returns true if this MultiReport wants to handle the given Test.
func (m MultiReport) wants(test *Test) (bool, error) {
	return test.ID.Match(m.ID)
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

// multiRunner runs multiReporters.
type multiRunner struct {
	multi   []MultiReport
	started []bool
}

// newMultiRunner returns a new multiRunner.
func newMultiRunner(mr []MultiReport) *multiRunner {
	return &multiRunner{
		mr,
		make([]bool, len(mr)),
	}
}

// start calls any multiStarters among the multiReporters. If an error occurs,
// any successfully started multiReporters will be stopped when stop is called.
// If a multiReporter does not implement multiStarter, it is considered started.
func (m *multiRunner) start(work resultRW) (err error) {
	for i, mr := range m.multi {
		r := mr.multiReporter()
		if s, ok := r.(multiStarter); ok {
			if err = s.start(work); err != nil {
				return
			}
		}
		m.started[i] = true
	}
	return
}

// tee confines goroutines to run all multiReporters for the given Test.
//
// The given Context may be used for handling cancellation. The resultRW may be
// used to write results to the working directory.
//
// Data items must be written to the returned out channel, which may be nil in
// case no multiReporters will run for the given Test. The out channel must be
// closed by the caller after use.
//
// The returned error channel receives any errors that occur, and may be nil in
// case no multiReporters will run for the given Test. If not nil, it is closed
// when the tee is done, meaning all of the multiReporters are done.
func (m *multiRunner) tee(ctx context.Context, work resultRW, test *Test) (
	out chan<- any, errc <-chan error) {
	// find multiReporters to run, returning nil out chan if there are none
	var rr []multiReporter
	for _, r := range m.multi {
		w, e := r.wants(test)
		if e != nil {
			ec := make(chan error, 1)
			ec <- e
			close(ec)
			errc = ec
			return
		}
		if w {
			rr = append(rr, r.multiReporter())
		}
	}
	if len(rr) == 0 {
		return
	}
	// create out channel, and data channels for multiReporters
	oc := make(chan any, dataChanBufLen)
	out = oc
	var dc []chan any
	for range rr {
		dc = append(dc, make(chan any, dataChanBufLen))
	}
	// start tee goroutine to read from out and write to data channels
	go func() {
		defer func() {
			for _, c := range dc {
				close(c)
			}
		}()
		for d := range oc {
			for _, c := range dc {
				c <- d
			}
		}
	}()
	// start goroutines for each multiReporter to call its report method
	var mec errChans
	for i, r := range rr {
		ec := mec.make()
		go func(r multiReporter, ec chan error) {
			defer func() {
				for range dc[i] {
				}
				close(ec)
			}()
			if e := r.report(ctx, work, test, dc[i]); e != nil {
				ec <- e
			}
		}(r, ec)
	}
	errc = mec.merge()
	return
}

// stop calls any multiStoppers among the multiReporters, and returns the first
// error, if any. stop is called on all multiReporters regardless of errors.
func (m *multiRunner) stop(work resultRW) (err error) {
	for i, mr := range m.multi {
		r := mr.multiReporter()
		if s, ok := r.(multiStopper); ok && m.started[i] {
			if e := s.stop(work); e != nil && err == nil {
				err = e
			}
		}
	}
	return
}
