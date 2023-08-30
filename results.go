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

// root returns a resultRW with RootDir as the prefix.
func (r Results) root() resultRW {
	return resultRW{r.RootDir, r.Destructive}
}

// work returns a resultRW with WorkDir as the prefix.
func (r Results) work() resultRW {
	return resultRW{r.WorkDir, r.Destructive}
}

// resultRW provides a rwer implementation for a given path prefix. If
// destructive is true, the Writer method overwrites existing results.
type resultRW struct {
	prefix      string
	destructive bool
}

// Join returns a new resultRW by joining the prefix of this resultRW with the
// given prefix using filepath.Join.
func (r resultRW) Join(prefix string) resultRW {
	return resultRW{
		filepath.Clean(filepath.Join(r.prefix, prefix)),
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
	if d := filepath.Dir(p); d != "/" && d != "." && d != ".." {
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
