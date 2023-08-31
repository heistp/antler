// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
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

// do calls the given doer for this Test.
//
// TODO check if Test.do can be factored out
func (t *Test) do(ctx context.Context, d doer, rst reportStack) (err error) {
	return d.do(ctx, t, rst)
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
	wc, err = t.WorkRW(res).Writer(t.DataFile)
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

// NoDataFileError is returned by DataWriter ot DataReader when the Test's
// DataFile field is empty, so no data may be read or written. The Test field
// is the corresponding Test.
type NoDataFileError struct {
	Test *Test
}

// Error implements error
func (n NoDataFileError) Error() string {
	return fmt.Sprintf("DataFile field is empty for: '%s'\n", n.Test.ID)
}

// WorkRW returns a resultRW for reading and writing this Test's results in the
// working directory.
func (t *Test) WorkRW(res Results) resultRW {
	return res.work().Append(t.ResultPrefixX)
}
