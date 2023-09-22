// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"encoding/gob"
	"fmt"
	"io"
	"maps"
	"sort"
	"strings"

	"github.com/heistp/antler/node"
)

// Test is an Antler test.
type Test struct {
	// ID uniquely identifies the Test in the test package.
	ID TestID

	// ResultPrefix is the path prefix for result files. It may use Go template
	// syntax, and is further documented in config.cue.
	ResultPrefix string

	// DataFile is the name of the gob file containing the raw result data. If
	// empty, raw result data is not saved for the Test.
	DataFile string

	// Run is the top-level Run instance.
	node.Run

	// During contains Reports to be run while the Test is run.
	During Report

	// Report contains Reports to be run after the Test is run.
	Report Report

	// ResultPrefixX contains the output of the executed ResultPrefix template.
	ResultPrefixX string
}

// TestID represents a compound Test identifier. Keys and values must match the
// regex defined in config.cue.
type TestID map[string]string

// Equal returns true if other is equal to this TestID (they contain the same
// key/value pairs).
func (i TestID) Equal(other TestID) bool {
	return maps.Equal(i, other)
}

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

// DataWriter returns a WriteCloser for writing result data to the work
// directory.
//
// If DataFile is empty, NoDataFileError is returned.
func (t *Test) DataWriter(res Results) (wc io.WriteCloser, err error) {
	if t.DataFile == "" {
		err = NoDataFileError{t}
		return
	}
	wc = t.WorkRW(res).Writer(t.DataFile)
	return
}

// DataReader returns a ReadCloser for reading result data.
//
// If DataFile is empty, NoDataFileError is returned.
//
// If the data file does not exist, errors.Is(err, fs.ErrNotExist) returns true.
func (t *Test) DataReader(res Results) (rc io.ReadCloser, err error) {
	if t.DataFile == "" {
		err = NoDataFileError{t}
		return
	}
	rc, err = t.WorkRW(res).Reader(t.DataFile)
	return
}

// NoDataFileError is returned by DataWriter or DataReader when the Test's
// DataFile field is empty, so no data may be read or written. The Test field
// is the corresponding Test.
type NoDataFileError struct {
	Test *Test
}

// Error implements error
func (n NoDataFileError) Error() string {
	return fmt.Sprintf("DataFile field is empty for: '%s'\n", n.Test.ID)
}

// DataHasError returns true if the DataFile exists and has errors. See
// DataReader for the errors that may be returned.
func (t *Test) DataHasError(res Results) (hasError bool, err error) {
	var r io.ReadCloser
	if r, err = t.DataReader(res); err != nil {
		return
	}
	defer func() {
		if e := r.Close(); e != nil && err == nil {
			err = e
		}
	}()
	c := gob.NewDecoder(r)
	for {
		var a any
		if err = c.Decode(&a); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		if _, ok := a.(error); ok {
			hasError = true
			return
		}
	}
}

// WorkRW returns a resultRW for reading and writing this Test's results in the
// working directory.
func (t *Test) WorkRW(res Results) resultRW {
	return res.work().Append(t.ResultPrefixX)
}

// LinkPriorData creates hard links for the result data for this Test from the
// prior (latest) result directory, to the working directory. DataFile is
// linked, along with any FileRefs it contains. If there was no prior result or
// no data file for this Test, then errors.Is(err, fs.ErrNotExist) will return
// true.
//
// If DataFile is empty, NoDataFileError is returned.
func (t *Test) LinkPriorData(res Results) (err error) {
	if t.DataFile == "" {
		err = NoDataFileError{t}
		return
	}
	if err = t.LinkPrior(res, t.DataFile); err != nil {
		return
	}
	var r io.ReadCloser
	if r, err = t.DataReader(res); err != nil {
		return
	}
	defer func() {
		if e := r.Close(); e != nil && err == nil {
			err = e
		}
	}()
	c := gob.NewDecoder(r)
	for {
		var a any
		if err = c.Decode(&a); err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		if l, ok := a.(FileRef); ok {
			if err = t.LinkPrior(res, l.Name); err != nil {
				return
			}
		}
	}
	return
}

// LinkPrior creates a hard link for the named result file for this Test from
// the prior (latest) result directory, to the working directory. If there were
// no prior results, or no prior named result file for this Test, then
// errors.Is(err, fs.ErrNotExist) will return true.
func (t *Test) LinkPrior(res Results, name string) (err error) {
	var l resultRW
	if l, err = res.prior(); err != nil {
		return
	}
	l = l.Append(t.ResultPrefixX)
	err = t.WorkRW(res).Link(l, name)
	return
}
