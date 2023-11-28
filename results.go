// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Results configures the behavior for reading and writing result files, which
// include all output files and reports.
//
// Callers must use the open method to obtain a resultRW to read and write
// results in WorkDir. See the doc on resultRW for more info.
type Results struct {
	RootDir         string
	WorkDir         string
	ResultDirUTC    bool
	ResultDirFormat string
	LatestSymlink   string
	Codec           Codecs
}

// open returns a new resultRW for reading and writing results to WorkDir.
// The existence of WorkDir is used as a lock to prevent multiple antler
// instances from writing results at the same time.
func (r Results) open() (rw resultRW, err error) {
	d := filepath.Dir(r.WorkDir)
	if d != "." && d != ".." && d != "/" {
		if err = os.MkdirAll(d, 0755); err != nil {
			return
		}
	}
	if err = os.Mkdir(r.WorkDir, 0755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			err = fmt.Errorf("'%s' exists- move it away if not in use (%w)",
				r.WorkDir, err)
		}
		return
	}
	var i []ResultInfo
	if i, err = r.info(); err != nil {
		return
	}
	rw = resultRW{r, "", i, &resultStat{}}
	return
}

// info returns a list of ResultInfos by reading the directory names under
// RootDir that match ResultDirFormat. The returned ResultInfos are sorted
// descending by Name. If RootDir does not exist, len(ii) is 0 and err is nil.
func (r Results) info() (ii []ResultInfo, err error) {
	var d *os.File
	if d, err = os.Open(r.RootDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
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

// byID returns a slice of the Codecs, sorted ascending by ID.
func (s Codecs) byID() (cc []Codec) {
	for _, v := range s {
		cc = append(cc, v)
	}
	sort.Slice(cc, func(i, j int) bool {
		return cc[i].ID < cc[j].ID
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
	return c.ID == other.ID &&
		slices.Equal(c.Extension, other.Extension) &&
		c.Encode == other.Encode &&
		slices.Equal(c.EncodeArg, other.EncodeArg) &&
		c.EncodePriority == other.EncodePriority &&
		c.Decode == other.Decode &&
		slices.Equal(c.DecodeArg, other.DecodeArg) &&
		c.DecodePriority == other.DecodePriority
}

// ResultInfo contains information on one result.
type ResultInfo struct {
	Name string // base name of result directory
	Path string // path to result directory
}

// resultRW provides access to read and write result files in WorkDir. When
// callers are done, they must either call Close or Abort, to finalize or
// abandon the result, respectively. Use of a defer statement is strongly
// advised. If no unique results were written at the time of Close, Abort is
// called automatically.
type resultRW struct {
	Results
	prefix string
	info   []ResultInfo
	stat   *resultStat
}

// resultStat records statistics on the reading and writing of results.
type resultStat struct {
	sync.Mutex
	WrittenFiles int
	LinkedFiles  int
	RemovedFiles int
}

// AddWrittenFiles adds n written files.
func (s *resultStat) AddWrittenFiles(n int) {
	s.Lock()
	s.WrittenFiles += n
	s.Unlock()
}

// RemoveWrittenFiles removes n written files.
func (s *resultStat) RemoveWrittenFiles(n int) {
	s.Lock()
	s.WrittenFiles -= n
	s.Unlock()
}

// AddLinkedFiles adds n linked files.
func (s *resultStat) AddLinkedFiles(n int) {
	s.Lock()
	s.LinkedFiles += n
	s.Unlock()
}

// AddRemovedFiles adds n removed files.
func (s *resultStat) AddRemovedFiles(n int) {
	s.Lock()
	s.RemovedFiles += n
	s.Unlock()
}

// Changed returns true if any files were written or removed.
func (s *resultStat) Changed() (changed bool) {
	s.Lock()
	changed = s.WrittenFiles > 0 || s.RemovedFiles > 0
	s.Unlock()
	return
}

// Child returns a child resultRW by appending the given prefix to the prefix
// of this resultRW.
func (r resultRW) Child(prefix string) resultRW {
	return resultRW{r.Results, r.prefix + prefix, r.info, r.stat}
}

// Reader implements rwer
func (r resultRW) Reader(name string) (*ResultReader, error) {
	return newResultReader(name, r.path(name), r.Codec)
}

// Writer implements rwer. The written file may be transparently encoded, if
// name's extension belongs to a registered Codec.
func (r resultRW) Writer(name string) (w *ResultWriter) {
	w = &ResultWriter{
		Name: name,
		Path: r.path(name),
	}
	if name == "-" {
		w.WriteCloser = stdoutWriter{}
		w.initted = true
		return
	}
	w.WriteCloser = newAtomicWriter(r.prefix+name, r.WorkDir, r.info, r.stat)
	var ok bool
	if w.Codec, ok = r.Codec.forName(name); !ok {
		return
	}
	w.WriteCloser = newCmdWriter(w.Codec.encodeCmd(), w.WriteCloser)
	return
}

// Remove implements rwer.
func (r resultRW) Remove(name string) (err error) {
	if err = os.Remove(name); err == nil {
		r.stat.AddRemovedFiles(1)
	}
	return
}

// Link creates hard links, for all encodings, for the named file from the most
// recent prior result containing name in any encoding. If no source was found
// to link the file, LinkError is returned.
func (r resultRW) Link(name string) (err error) {
	var xx []string
	xx = append(xx, "")
	for _, c := range r.Codec.byID() {
		xx = append(xx, c.Extension...)
	}
	n := r.prefix + name
	var ok bool
	for i := 0; i < len(r.info) && !ok; i++ {
		w := filepath.Join(r.WorkDir, n)
		p := filepath.Join(r.info[i].Path, n)
		for _, x := range xx {
			if _, e := os.Stat(p + x); e != nil {
				if !errors.Is(e, fs.ErrNotExist) {
					return
				}
				continue
			}
			if err = os.MkdirAll(filepath.Dir(w+x), 0755); err != nil {
				return
			}
			if err = os.Link(p+x, w+x); err != nil {
				return
			}
			r.stat.AddLinkedFiles(1)
			ok = true
		}
	}
	if !ok {
		err = LinkError{n}
	}
	return
}

// LinkError is returned by resultRW.Link when the named file could not be found
// in any prior result.
type LinkError struct {
	Name string
}

// Error implements error.
func (l LinkError) Error() string {
	return fmt.Sprintf("prior file not found for link: '%s'", l.Name)
}

// Is makes this error an fs.ErrNotExist for the errors package.
func (l LinkError) Is(target error) bool {
	return target == fs.ErrNotExist
}

// Close finalizes the result by renaming WorkDir to the final result directory
// (resultDir return parameter), and updating the latest symlink. If WorkDir
// and/or RootDir are empty because no results changed, they are removed,
// and no error is returned as long as this succeeds. If no unique files were
// written, Abort is called instead.
func (r resultRW) Close() (resultDir string, err error) {
	if !r.stat.Changed() {
		err = r.Abort()
		return
	}
	var y bool
	if y, err = dirEmpty(r.WorkDir); err != nil {
		return
	}
	if y {
		if err = os.Remove(r.WorkDir); err != nil {
			return
		}
		var x bool
		if x, err = dirEmpty(r.RootDir); err != nil {
			return
		}
		if x {
			err = os.Remove(r.RootDir)
		}
		return
	}
	t := time.Now()
	if r.ResultDirUTC {
		t = t.UTC()
	}
	n := t.Format(r.ResultDirFormat)
	resultDir = filepath.Join(r.RootDir, n)
	if err = os.Rename(r.WorkDir, resultDir); errors.Is(err, fs.ErrNotExist) {
		err = nil
		return
	}
	if r.LatestSymlink != "" {
		l := r.LatestSymlink + "~"
		if err = os.Symlink(n, l); err != nil {
			return
		}
		err = os.Rename(l, r.LatestSymlink)
	}
	return
}

// dirEmpty returns empty true if the named directory is empty or does not exist.
func dirEmpty(name string) (empty bool, err error) {
	var d *os.File
	if d, err = os.Open(name); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return
	}
	defer func() {
		if e := d.Close(); e != nil && err == nil {
			err = e
		}
	}()
	if _, err = d.Readdirnames(1); err == io.EOF {
		empty = true
		err = nil
	}
	return
}

// Abort removes WorkDir and its contents, thereby aborting a result. If RootDir
// is then empty, it is also removed.
func (r resultRW) Abort() (err error) {
	if err = os.RemoveAll(r.WorkDir); err != nil {
		return
	}
	var x bool
	if x, err = dirEmpty(r.RootDir); err != nil {
		return
	}
	if x {
		err = os.Remove(r.RootDir)
	}
	return
}

// path returns the path to a results file given its name.
func (r resultRW) path(name string) string {
	if name == "-" {
		return "-"
	}
	return filepath.Join(r.WorkDir, r.prefix+name)
}

// rwer provides methods to read and write results.
type rwer interface {
	// Reader returns a ResultReader for reading the named result file. Callers
	// should take care to always close the returned ResultReader.
	Reader(name string) (*ResultReader, error)

	// Writer returns a ResultWriter for writing a result. If name is "-", the
	// result is written to stdout. Otherwise, the result is written to the
	// named result file. Callers should take care to always close the returned
	// ResultWriter.
	Writer(name string) *ResultWriter

	// Remove calls os.Remove to remove the named file or directory.
	Remove(name string) error
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
// file could be found, an os.PathError is returned, and
// errors.Is(err, fs.ErrNotExist) will return true.
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
	err = &os.PathError{
		Op:   "reader",
		Path: path,
		Err:  fs.ErrNotExist,
	}
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
				e = fmt.Errorf("cmdReader stdin close error: %w", ce)
			}
			if e != nil {
				r.errc <- e
			}
			close(r.errc)
		}()
		if _, e = io.Copy(i, r.underlying); e != nil {
			e = fmt.Errorf("cmdReader io.Copy error: %w", e)
		}
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
			err = fmt.Errorf("%s: %w", r.cmd, e)
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

// ResultWriter writes a result file.
type ResultWriter struct {
	// Name is the name of the result file as requested. This does not
	// correspond to the name of a file on the filesystem.
	Name string

	// Path is the path to the result file actually written, including WorkDir
	// and the result prefix.
	Path string

	// Codec is the Codec used to encode the file (based on Name's extension).
	// The zero value of Codec means the file is written directly.
	Codec Codec

	// WriteCloser writes the result file, encoding it transparently if needed.
	io.WriteCloser

	// initted is true after ResultWriter is lazily initialized in Write.
	initted bool
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
				w.errc <- fmt.Errorf("cmdWriter io.Copy error: %w", e)
			}
			close(w.errc)
		}()
		if _, e = io.Copy(w.underlying, o); e != nil {
			e = fmt.Errorf("cmdWriter io.Copy error: %w", e)
		}
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
			err = fmt.Errorf("%s: %w", w.cmd, e)
		}
	}
	if e := w.underlying.Close(); e != nil && err == nil {
		err = e
	}
	return
}

// atomicWriter is a WriteCloser for a given named file that first writes to a
// temporary file name~, then when Close is called, either hard links name from
// a prior version if it's the same, or moves name~ to name. It is strongly
// suggested to call Close in a defer, and to check for any errors it may
// return.
//
// The temporary file name~ is lazily created by Write. If Write is not called
// at all, the file is never created, and nothing happens on Close.
type atomicWriter struct {
	name    string // includes prefix, but not WorkDir
	workDir string
	info    []ResultInfo
	tmp     *os.File
	stat    *resultStat
}

// newAtomicWriter returns a new atomicWriter.
func newAtomicWriter(name, workDir string, info []ResultInfo,
	stat *resultStat) *atomicWriter {
	return &atomicWriter{name, workDir, info, nil, stat}
}

// path returns the path to the file in WorkDir.
func (a *atomicWriter) path() string {
	return filepath.Join(a.workDir, a.name)
}

// tmpPath returns the path to the temporary file for writing in WorkDir.
func (a *atomicWriter) tmpPath() string {
	return filepath.Join(a.workDir, a.name+"~")
}

// Write implements io.Writer.
func (a *atomicWriter) Write(p []byte) (n int, err error) {
	if a.tmp == nil {
		if a.tmp, err = os.Create(a.tmpPath()); err != nil {
			return
		}
		a.stat.AddWrittenFiles(1)
	}
	n, err = a.tmp.Write(p)
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
	var p string
	if p, err = a.findPrior(); err != nil {
		return
	}
	if e := os.Remove(a.path()); e != nil && !errors.Is(e, fs.ErrNotExist) {
		err = e
		return
	}
	if p != "" {
		if err = os.Link(p, a.path()); err != nil {
			return
		}
		a.stat.RemoveWrittenFiles(1)
		a.stat.AddLinkedFiles(1)
		err = os.Remove(a.tmpPath())
	} else {
		err = os.Rename(a.tmpPath(), a.path())
	}
	return
}

// findPrior searches for a file with the same name and contents in the prior
// result. If not found, an empty path is returned and err is nil.
func (a *atomicWriter) findPrior() (path string, err error) {
	if len(a.info) > 0 {
		i := a.info[0]
		path = filepath.Join(i.Path, a.name)
		var s bool
		if s, err = compareFiles(a.tmpPath(), path); err != nil || s {
			return
		}
	}
	path = ""
	return
}

// compareFiles returns true if both name1 and name2 exist, and have the same
// size and contents.
func compareFiles(name1, name2 string) (same bool, err error) {
	var i1, i2 os.FileInfo
	if i1, err = os.Stat(name1); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return
	}
	if i2, err = os.Stat(name2); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return
	}
	if same = i1.Size() == i2.Size(); !same {
		return
	}
	var f1, f2 *os.File
	if f1, err = os.Open(name1); err != nil {
		return
	}
	defer f1.Close()
	if f2, err = os.Open(name2); err != nil {
		return
	}
	defer f2.Close()
	r1 := bufio.NewReaderSize(f1, 64*1024)
	r2 := bufio.NewReaderSize(f2, 64*1024)
	same = true
	var d1, d2 bool
	for {
		var b1, b2 byte
		if b1, err = r1.ReadByte(); err != nil {
			if err != io.EOF {
				return
			}
			d1 = true
			err = nil
		}
		if b2, err = r2.ReadByte(); err != nil {
			if err != io.EOF {
				return
			}
			d2 = true
			err = nil
		}
		if d1 != d2 {
			same = false
			return
		}
		if d1 && d2 {
			return
		}
		if b1 != b2 {
			same = false
			return
		}
	}
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
