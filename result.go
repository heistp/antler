// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/heistp/antler/node"
)

// Result contains the Test that was run and Data from the node.
type Result struct {
	Test     Test
	Data     []node.DataPoint
	Log      []node.LogEntry
	Feedback node.Feedback
	Error    []node.Error
	file     map[string]*os.File
}

// newResult creates and returns a new Result.
func newResult(t Test) Result {
	return Result{
		t,
		make([]node.DataPoint, 0),
		make([]node.LogEntry, 0),
		node.Feedback{},
		make([]node.Error, 0),
		make(map[string]*os.File),
	}
}

// gather reads result items from the given channel and adds them to the Result.
// After the channel is closed, the items are sorted before gather closes its
// done channel and returns.
func (r *Result) gather(result <-chan interface{}, done chan struct{}) {
	defer close(done)
	defer func() {
		for _, f := range r.file {
			f.Close()
		}
	}()
	for i := range result {
		switch v := i.(type) {
		case node.DataPoint:
			r.Data = append(r.Data, v)
		case node.FileData:
			r.writeFileData(v)
		case node.LogEntry:
			r.Log = append(r.Log, v)
		case node.Feedback:
			r.Feedback = v
		case node.Error:
			r.Error = append(r.Error, v)
		default:
			panic(fmt.Sprintf("gather received unknown result type: %T", i))
		}
	}
	sort.Slice(r.Data, func(i, j int) bool {
		return r.Data[i].Time.Time.Before(r.Data[j].Time.Time)
	})
	sort.Slice(r.Log, func(i, j int) bool {
		return r.Log[i].Time.Before(r.Log[j].Time)
	})
	sort.Slice(r.Error, func(i, j int) bool {
		return r.Error[i].Time.Before(r.Error[j].Time)
	})
}

// writeFileData opens files, as necessary, and appends file data.
func (r *Result) writeFileData(fd node.FileData) (err error) {
	var f *os.File
	var ok bool
	if f, ok = r.file[fd.Name]; !ok {
		if f, err = os.Create(fd.Name); err != nil {
			return
		}
		r.file[fd.Name] = f
	}
	_, err = f.Write(fd.Data)
	return
}

// DumpText emits a text representation of the Result for debugging purposes to
// the given Writer.
func (r *Result) DumpText(w io.Writer) {
	f := func(format string, a ...interface{}) {
		fmt.Fprintf(w, format, a...)
	}
	f("Test Props: %s\n", r.Test.Props)
	if len(r.Feedback) > 0 {
		f("\n")
		f("Feedback: %s\n", r.Feedback)
	}
	f("\n")
	f("Data Points (%d):\n", len(r.Data))
	for _, p := range r.Data {
		f("%s\n", p)
	}
	f("\n")
	f("Log Entries (%d):\n", len(r.Log))
	for _, l := range r.Log {
		f("%s\n", l)
	}
	if len(r.Error) > 0 {
		f("\n")
		f("Errors:\n")
		for _, e := range r.Error {
			f("%s\n", e)
		}
	}
}
