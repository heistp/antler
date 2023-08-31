// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Results configures the behavior for reading and writing result files, which
// include all output files and reports.
type Results struct {
	Destructive     bool
	RootDir         string
	WorkDir         string
	ResultDirUTC    bool
	ResultDirFormat string
}

// open prepares the Results for use, and must be called before other Results
// methods are used.
func (r Results) open() (err error) {
	if r.Destructive {
		return
	}
	if err = os.MkdirAll(r.RootDir, 0755); err != nil {
		return
	}
	if err = os.Mkdir(r.WorkDir, 0755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			err = fmt.Errorf(
				"directory '%s' exists- ensure no other test is running, then move it away",
				r.WorkDir)
		}
		return
	}
	return
}

// close finalizes the Results, and must be called after all results are
// written, preferably by a defer statement.
func (r Results) close() (err error) {
	if r.Destructive {
		return
	}
	t := time.Now()
	if r.ResultDirUTC {
		t = t.UTC()
	}
	n := t.Format(r.ResultDirFormat)
	err = os.Rename(r.WorkDir, filepath.Join(r.RootDir, n))
	return
}

// root returns a resultRW with RootDir as the prefix.
func (r Results) root() resultRW {
	return resultRW{r.RootDir + string(os.PathSeparator), r.Destructive}
}

// work returns a resultRW with WorkDir as the prefix.
func (r Results) work() resultRW {
	return resultRW{r.WorkDir + string(os.PathSeparator), r.Destructive}
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

// ResultInfo contains information on one result.
type ResultInfo struct {
	Name string // base name of result directory
	Path string // path to result directory
}

// resultRW provides a rwer implementation for a given path prefix. If
// destructive is true, the Writer method overwrites existing results.
type resultRW struct {
	prefix      string
	destructive bool
}

// Append returns a new resultRW by appending the given prefix to the prefix of
// this resultRW.
func (r resultRW) Append(prefix string) resultRW {
	return resultRW{
		r.prefix + prefix,
		r.destructive,
	}
}

// Reader implements rwer
func (r resultRW) Reader(name string) (io.ReadCloser, error) {
	return os.Open(r.path(name))
}

// Writer implements rwer
func (r resultRW) Writer(name string) (wc io.WriteCloser, err error) {
	if name == "-" {
		wc = &stdoutWriter{}
		return
	}
	p := r.path(name)
	if !r.destructive {
		if _, err = os.Stat(p); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return
			}
		} else {
			err = FileExistsError{p}
			return
		}
	}
	if d := filepath.Dir(p); d != string(os.PathSeparator) &&
		d != "." && d != ".." {
		if err = os.MkdirAll(d, 0755); err != nil {
			return
		}
	}
	wc, err = os.Create(p)
	return
}

// path returns the path to a results file given its name.
func (r resultRW) path(name string) string {
	return filepath.Clean(r.prefix + name)
}

// FileExistsError is returned by Writer when the named file already exists, and
// destructive is false. The Path field is the path to the file.
type FileExistsError struct {
	Path string
}

// Error implements error
func (f FileExistsError) Error() string {
	return fmt.Sprintf("file already exists: '%s'\n", f.Path)
}

// readerer wraps the Reader method, to return a ReadCloser for reading results.
// The name parameter identifies the result data according to the underlying
// implementation, and is typically a filename, or filename suffix.
type readerer interface {
	Reader(name string) (io.ReadCloser, error)
}

// writerer wraps the Writer method, to return a WriteCloser for writing
// results. The name parameter identifies the result data according to the
// underlying implementation, and is typically a filename, or filename suffix.
type writerer interface {
	Writer(name string) (io.WriteCloser, error)
}

// rwer groups the readerer and writerer interfaces.
type rwer interface {
	readerer
	writerer
}

// stdoutWriter is a WriteCloser that writes to stdout. The Close implementation
// does nothing.
type stdoutWriter struct {
}

// Write implements io.Writer
func (stdoutWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

// Close implements io.Closer
func (stdoutWriter) Close() error {
	return nil
}
