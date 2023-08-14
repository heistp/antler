// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/heistp/antler/node"
)

// Test is an Antler test.
type Test struct {
	// ID uniquely identifies the Test in the test package.
	ID TestID

	// OutPathTemplate is a Go template used to generate the base path for
	// output files.
	OutPathTemplate string

	// DataFile is the name of the gob output file containing the raw result
	// data. If empty, raw result data is not saved for the Test.
	DataFile string

	// Run is the top-level Run instance.
	node.Run

	// Report lists Reports to be run on this Test.
	Report reports
}

// TestID is a compound Test identifier. Keys and values must match the regex
// defined in config.cue.
type TestID map[string]string

// String returns a canonical version of the Test ID in the form:
// [K=V ...] with key/value pairs sorted by their keys.
func (i TestID) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[")
	kk := make([]string, 0, len(i))
	for k := range i {
		kk = append(kk, k)
	}
	sort.Strings(kk)
	for j, k := range kk {
		if j > 0 {
			fmt.Fprintf(&b, " ")
		}
		fmt.Fprintf(&b, "%s=%s", k, i[k])
	}
	fmt.Fprintf(&b, "]")
	return b.String()
}

// do pushes the Test Reports to the stack, and calls the given doer.
func (t *Test) do(d doer, rst reporterStack) (err error) {
	rst.push(t.Report.reporters())
	defer rst.pop()
	return d.do(t, rst)
}

// DataWriter returns a WriteCloser for writing result data.
//
// The overwrite parameter indicates whether to overwrite existing data (if
// true), or not (if false), in which case FileExistsError is returned if
// the file referred to by DataFile already exists.
//
// If DataFile is empty, NoDataFileError is returned.
func (t *Test) DataWriter(overwrite bool) (wc io.WriteCloser, err error) {
	if t.DataFile == "" {
		err = &NoDataFileError{t}
		return
	}
	wc, err = t.Writer(t.DataFile, overwrite)
	return
}

// DataReader returns a ReadCloser for reading result data.
//
// If DataFile is empty, NoDataFileError is returned.
//
// If the data file does not exist, errors.Is(err, fs.ErrNotExist) returns true.
func (t *Test) DataReader() (rc io.ReadCloser, err error) {
	if t.DataFile == "" {
		err = &NoDataFileError{t}
		return
	}
	var p string
	if p, err = t.outPath(t.DataFile); err != nil {
		return
	}
	rc, err = os.Open(p)
	return
}

// NoDataFileError is returned by DataWriter ot DataReader when the Test's
// DataFile field is empty, so no data may be read or written. The Test field
// is the corresponding Test.
type NoDataFileError struct {
	Test *Test
}

func (n *NoDataFileError) Error() string {
	return fmt.Sprintf("DataFile field is empty for: '%s'\n", n.Test.ID)
}

// Writer returns a WriteCloser for writing result data.
//
// The overwrite parameter indicates whether to overwrite an existing file (if
// true), or not (if false), in which case FileExistsError is returned if the
// file referred to by name already exists.
//
// Any directories in name are automatically created.
func (t *Test) Writer(name string, overwrite bool) (wc io.WriteCloser,
	err error) {
	var p string
	if p, err = t.outPath(name); err != nil {
		return
	}
	if !overwrite {
		if _, err = os.Stat(p); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return
			}
		} else {
			err = &FileExistsError{p}
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

// FileExistsError is returned by Writer when the named file already exists, and
// overwrite is false. The Path field is the path to the file.
type FileExistsError struct {
	Path string
}

func (f *FileExistsError) Error() string {
	return fmt.Sprintf("data file already exists: '%s'\n", f.Path)
}

// outPath returns the path to an output file, by appending the given name to
// the base output path generated by OutPathTemplate.
func (t *Test) outPath(name string) (path string, err error) {
	var m *template.Template
	if m, err = template.New("OutPath").Parse(t.OutPathTemplate); err != nil {
		return
	}
	var b strings.Builder
	if err = m.Execute(&b, t.ID); err != nil {
		return
	}
	path = b.String() + name
	return
}
