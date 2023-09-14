// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Results configures the behavior for reading and writing result files, which
// include all output files and reports.
type Results struct {
	RootDir         string
	WorkDir         string
	ResultDirUTC    bool
	ResultDirFormat string
	Codec           Codecs
}

// open ensures the Results are ready for use. It must be called before other
// Results methods are used.
func (r Results) open() error {
	var e error
	if _, e = os.Stat(r.WorkDir); e == nil {
		return fmt.Errorf(
			"'%s' already exists- ensure no other test is running, then move it away",
			r.WorkDir)
	}
	if errors.Is(e, fs.ErrNotExist) {
		return nil
	}
	return e
}

// close finalizes the Results, and must be called after all results are
// written. Use of a defer statement is strongly advised.
func (r Results) close() (err error) {
	t := time.Now()
	if r.ResultDirUTC {
		t = t.UTC()
	}
	p := filepath.Join(r.RootDir, t.Format(r.ResultDirFormat))
	if err = os.Rename(r.WorkDir, p); errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	return
}

// root returns a resultRW with RootDir as the prefix.
func (r Results) root() resultRW {
	return resultRW{r.RootDir + string(os.PathSeparator), r.Codec}
}

// work returns a resultRW with WorkDir as the prefix.
func (r Results) work() resultRW {
	return resultRW{r.WorkDir + string(os.PathSeparator), r.Codec}
}

// resultInfo returns a list of ResultInfos by reading the directory names under
// RootDir that match ResultDirFormat. The returned ResultInfos are sorted
// descending by Name.
func (r Results) resultInfo() (ii []ResultInfo, err error) {
	var d *os.File
	if d, err = os.Open(r.RootDir); err != nil {
		return
	}
	defer d.Close()
	var ee []fs.DirEntry
	if ee, err = d.ReadDir(0); err != nil {
		return
	}
	for _, e := range ee {
		var i fs.FileInfo
		if i, err = e.Info(); err != nil {
			return
		}
		n := i.Name()
		if _, te := time.Parse(r.ResultDirFormat, n); te == nil {
			ii = append(ii, ResultInfo{n, filepath.Join(r.RootDir, n)})
		}
	}
	sort.Slice(ii, func(i, j int) bool {
		return ii[i].Name > ii[j].Name
	})
	return
}

// Codecs wraps a map of Codecs to provide related methods.
type Codecs map[string]Codec

// byDecodePrio returns a slice of the Codecs, sorted ascending by
// DecodePriority.
func (s Codecs) byDecodePrio() (cc []Codec) {
	for _, v := range s {
		cc = append(cc, v)
	}
	sort.Slice(cc, func(i, j int) bool {
		return cc[i].DecodePriority < cc[j].DecodePriority
	})
	return
}

// byEncodePrio returns a slice of the Codecs, sorted ascending by
// EncodePriority.
func (s Codecs) byEncodePrio() (cc []Codec) {
	for _, v := range s {
		cc = append(cc, v)
	}
	sort.Slice(cc, func(i, j int) bool {
		return cc[i].EncodePriority < cc[j].EncodePriority
	})
	return
}

// forName returns a Codec for encoding the given filename. Ok is true if a
// Codec was found.
func (s Codecs) forName(name string) (cod Codec, ok bool) {
	for _, c := range s.byEncodePrio() {
		if c.handlesName(name) {
			cod = c
			ok = true
			return
		}
	}
	return
}

// Codec configures a file encoder/decoder.
type Codec struct {
	ID             string
	Extension      []string
	Encode         string
	EncodeArg      []string
	EncodePriority int
	Decode         string
	DecodeArg      []string
	DecodePriority int
}

// handlesName returns true if the given file name ends with one of the
// Codec's Extensions.
func (c Codec) handlesName(name string) bool {
	for _, x := range c.Extension {
		if strings.HasSuffix(name, x) {
			return true
		}
	}
	return false
}

// openEncoded opens an encoded version of the named file for reading. If no
// encoded version of the named file is found, f is nil.
func (c Codec) openEncoded(name string) (f *os.File, err error) {
	for _, x := range c.Extension {
		if f, err = os.Open(name + x); err == nil {
			return
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return
		}
	}
	err = nil
	return
}

// encodeCmd returns an exec.Cmd that encodes data from stdin to stdout.
func (c Codec) encodeCmd() *exec.Cmd {
	return exec.Command(c.Encode, c.EncodeArg...)
}

// decodeCmd returns an exec.Cmd that decodes data from stdin to stdout.
func (c Codec) decodeCmd() *exec.Cmd {
	return exec.Command(c.Decode, c.DecodeArg...)
}

// Equal returns true if the Codecs are equal.
func (c Codec) Equal(other Codec) bool {
	return c.ID == other.ID
}

// ResultInfo contains information on one result.
type ResultInfo struct {
	Name string // base name of result directory
	Path string // path to result directory
}

// resultRW provides a rwer implementation for a given path prefix.
type resultRW struct {
	prefix string
	codec  Codecs
}

// Append returns a new resultRW by appending the given prefix to the prefix of
// this resultRW.
func (r resultRW) Append(prefix string) resultRW {
	return resultRW{r.prefix + prefix, r.codec}
}

// Reader implements rwer
func (r resultRW) Reader(name string) (io.ReadCloser, error) {
	return newResultReader(name, r.path(name), r.codec)
}

// rwer provides methods to read and write results.
type rwer interface {
	// Reader returns a ReadCloser of type *ResultReader for reading the named
	// result file.
	Reader(name string) (io.ReadCloser, error)

	// Writer returns a WriteCloser for writing a result. If name is "-", the
	// result is written to stdout. Otherwise, the result is written to the
	// named result file, and the returned WriteCloser is of type *ResultWriter.
	Writer(name string) (io.WriteCloser, error)
}

// ResultReader reads a result file.
type ResultReader struct {
	// Name is the name of the result file as requested. This is not the name of
	// a file on the filesystem.
	Name string

	// Path is the path to the result file actually read, which may be either an
	// encoded or unencoded version of the file.
	Path string

	// Codec is the Codec used to decode the file. The zero value of Codec means
	// the file is read directly.
	Codec Codec

	// ReadCloser reads the result file, decoding it transparently if needed.
	io.ReadCloser
}

// newResultReader returns a new ResultReader for a result file with the given
// name and path, transparently decoding the file if necessary. If the result
// file could be found, errors.Is(err, fs.ErrNotExist) will return true.
func newResultReader(name, path string, codec Codecs) (r *ResultReader,
	err error) {
	r = &ResultReader{
		Name: name,
		Path: path,
	}
	var f *os.File
	if f, err = os.Open(path); err == nil {
		r.ReadCloser = f
		return
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return
	}
	for _, c := range codec.byDecodePrio() {
		var f *os.File
		if f, err = c.openEncoded(path); err != nil {
			return
		}
		if f == nil {
			continue
		}
		r.Codec = c
		r.Path = f.Name()
		r.ReadCloser = newCmdReader(c.decodeCmd(), f)
		return
	}
	err = fmt.Errorf("%w: '%s'", fs.ErrNotExist, path)
	return
}

// cmdReader is a ReadCloser that uses a system command to filter data read from
// an underlying Reader. When cmdReader is closed, the underlying Reader is
// closed. cmdReader is not safe for concurrent use.
type cmdReader struct {
	cmd        *exec.Cmd
	underlying *truncateReader
	started    bool
	errc       chan error
	stdout     io.ReadCloser
}

// newCmdReader returns a new cmdReader, with the command started and the Reader
// ready for use.
func newCmdReader(cmd *exec.Cmd, underlying io.ReadCloser) (cr *cmdReader) {
	return &cmdReader{
		cmd,
		newTruncateReader(underlying),
		false,
		make(chan error, 1),
		nil,
	}
}

// Read implements io.Reader.
func (r *cmdReader) Read(p []byte) (n int, err error) {
	if !r.started {
		if err = r.start(); err != nil {
			return
		}
		r.started = true
	}
	n, err = r.stdout.Read(p)
	return
}

// start starts the command and goroutines.
func (r *cmdReader) start() (err error) {
	if r.stdout, err = r.cmd.StdoutPipe(); err != nil {
		return
	}
	var i io.WriteCloser
	if i, err = r.cmd.StdinPipe(); err != nil {
		return
	}
	go func() {
		var e error
		defer func() {
			if ce := i.Close(); ce != nil && e == nil {
				e = ce
			}
			if e != nil {
				r.errc <- e
			}
			close(r.errc)
		}()
		_, e = io.Copy(i, r.underlying)
	}()
	err = r.cmd.Start()
	return
}

// Close implements io.Closer.
func (r *cmdReader) Close() (err error) {
	if r.started {
		r.underlying.truncate()
		if _, e := io.Copy(io.Discard, r); e != nil && err == nil {
			err = e
		}
		if e := <-r.errc; e != nil && err == nil {
			err = e
		}
		if e := r.cmd.Wait(); e != nil && err == nil {
			err = e
		}
	}
	if e := r.underlying.Close(); e != nil && err == nil {
		err = e
	}
	return
}

// truncateReader wraps an underlying ReadCloser to provide a truncate method
// that causes any further Read calls to return 0, io.EOF.
type truncateReader struct {
	io.ReadCloser
	truncated bool
	mtx       sync.Mutex
}

// newTruncateReader returns a new truncateReader for the given underlying
// ReadCloser.
func newTruncateReader(underlying io.ReadCloser) *truncateReader {
	return &truncateReader{underlying, false, sync.Mutex{}}
}

// truncate causes any further calls to Read to return 0, io.EOF.
func (t *truncateReader) truncate() {
	t.mtx.Lock()
	t.truncated = true
	t.mtx.Unlock()
}

// Read implements io.Reader.
func (t *truncateReader) Read(p []byte) (n int, err error) {
	t.mtx.Lock()
	if t.truncated {
		err = io.EOF
		t.mtx.Unlock()
		return
	}
	t.mtx.Unlock()
	n, err = t.ReadCloser.Read(p)
	return
}

// Writer implements rwer
func (r resultRW) Writer(name string) (wc io.WriteCloser, err error) {
	if name == "-" {
		wc = &stdoutWriter{}
		return
	}
	wc = newResultWriter(name, r.path(name), r.codec)
	return
}

// ResultWriter writes a result file.
type ResultWriter struct {
	// Name is the name of the result file as requested. This does not
	// correspond to the name of a file on the filesystem.
	Name string

	// Path is the path to the result file actually written, including the
	// result prefix.
	Path string

	// Codec is the Codec used to encode the file (based on Name's extension).
	// The zero value of Codec means the file is written directly.
	Codec Codec

	// WriteCloser writes the result file, encoding it transparently if needed.
	io.WriteCloser

	// initted is true after ResultWriter is lazily initialized in Write.
	initted bool
}

// newResultWriter returns a new ResultWriter for a result file with the given
// name and path, transparently encoding the file if necessary.
func newResultWriter(name, path string, codec Codecs) (w *ResultWriter) {
	w = &ResultWriter{
		Name: name,
		Path: path,
	}
	w.WriteCloser = newAtomicWriter(path)
	var ok bool
	if w.Codec, ok = codec.forName(name); !ok {
		return
	}
	w.WriteCloser = newCmdWriter(w.Codec.encodeCmd(), w.WriteCloser)
	return
}

// Write implements io.Writer.
func (w *ResultWriter) Write(p []byte) (n int, err error) {
	if !w.initted {
		if err = os.MkdirAll(filepath.Dir(w.Path), 0755); err != nil {
			return
		}
		w.initted = true
	}
	n, err = w.WriteCloser.Write(p)
	return
}

// path returns the path to a results file given its name.
func (r resultRW) path(name string) string {
	return filepath.Clean(r.prefix + name)
}

// cmdWriter is a WriteCloser that uses a system command to filter data before
// writing it to the underlying Writer. When the cmdWriter is closed, the
// underlying Writer is also closed, after the command exits.
type cmdWriter struct {
	cmd        *exec.Cmd
	underlying io.WriteCloser
	errc       chan error
	started    bool
	io.WriteCloser
}

// newCmdWriter returns a new cmdWriter, with the command started and the Writer
// ready for use.
func newCmdWriter(cmd *exec.Cmd, underlying io.WriteCloser) *cmdWriter {
	// TODO lazily start cmdWriter
	return &cmdWriter{cmd, underlying, make(chan error, 1), false, nil}
}

// Write implements io.Writer.
func (w *cmdWriter) Write(p []byte) (n int, err error) {
	if !w.started {
		if err = w.start(); err != nil {
			return
		}
		w.started = true
	}
	n, err = w.WriteCloser.Write(p)
	return
}

// start starts the command and goroutines.
func (w *cmdWriter) start() (err error) {
	if w.WriteCloser, err = w.cmd.StdinPipe(); err != nil {
		return
	}
	var o io.ReadCloser
	if o, err = w.cmd.StdoutPipe(); err != nil {
		return
	}
	go func() {
		var e error
		defer func() {
			if e != nil {
				w.errc <- e
			}
			close(w.errc)
		}()
		_, e = io.Copy(w.underlying, o)
	}()
	err = w.cmd.Start()
	return
}

// Close implements io.Closer.
func (w *cmdWriter) Close() (err error) {
	if w.started {
		err = w.WriteCloser.Close()
		if e := <-w.errc; e != nil && err == nil {
			err = e
		}
		if e := w.cmd.Wait(); e != nil && err == nil {
			err = e
		}
	}
	if e := w.underlying.Close(); e != nil && err == nil {
		err = e
	}
	return
}

// atomicWriter is a WriteCloser for a given named file that first writes to a
// temporary file name~, then moves name~ to name when Close is called. It is
// strongly suggested to call Close in a defer, and to check for any errors it
// may return.
//
// The temporary file name~ is lazily created by Write. If Write is not called
// at all, the file is never created, and nothing happens on Close.
//
// For safety, if any errors occur on Write, then name~ is left in place, and
// not moved to name when Close is called.
//
// atomicWriter is not safe for concurrent use.
type atomicWriter struct {
	name string
	err  bool
	tmp  *os.File
}

// newAtomicWriter returns a new atomicWriter, open and ready for use.
func newAtomicWriter(name string) *atomicWriter {
	return &atomicWriter{name: name}
}

// tmpName returns the name of the temporary file for writing.
func (a *atomicWriter) tmpName() string {
	return a.name + "~"
}

// Write implements io.Writer.
func (a *atomicWriter) Write(p []byte) (n int, err error) {
	if a.tmp == nil {
		if a.tmp, err = os.Create(a.tmpName()); err != nil {
			return
		}
	}
	if n, err = a.tmp.Write(p); err != nil {
		a.err = true
	}
	return
}

// Close implements io.Closer.
func (a *atomicWriter) Close() (err error) {
	if a.tmp == nil {
		return
	}
	if err = a.tmp.Close(); err != nil {
		return
	}
	if a.err {
		return
	}
	err = os.Rename(a.tmpName(), a.name)
	return
}

// stdoutWriter writes to stdout, and does nothing on Close.
type stdoutWriter struct {
}

// Write implements io.Writer.
func (stdoutWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

// Close implements io.Closer.
func (stdoutWriter) Close() error {
	return nil
}
