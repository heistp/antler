// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/heistp/antler/node"
)

// EmitLog is a reporter that emits LogEntry's to files and/or stdout.
type EmitLog struct {
	// To lists the destinations to send output to. "-" sends output to stdout,
	// and everything else sends output to the named file. If To is empty,
	// output is emitted to stdout.
	To []string

	// Sort, if true, indicates to gather the logs, sort them by time, and emit
	// them after "in" is closed.
	Sort bool
}

// report implements reporter
func (l *EmitLog) report(ctx context.Context, in <-chan any, out chan<- any,
	rw rwer) (err error) {
	ww := []io.WriteCloser{stdoutWriter{}}
	defer func() {
		for _, w := range ww {
			w.Close()
		}
	}()
	if len(l.To) > 0 {
		ww = ww[:0]
		for _, s := range l.To {
			var w io.WriteCloser
			if w, err = rw.Writer(s, true); err != nil {
				return
			}
			ww = append(ww, w)
		}
	}
	emit := func(y node.LogEntry) error {
		for _, w := range ww {
			if _, e := fmt.Fprintln(w, y); e != nil {
				return e
			}
		}
		return nil
	}
	var yy []node.LogEntry
	for d := range in {
		out <- d
		if y, ok := d.(LogEntry); ok {
			if l.Sort {
				yy = append(yy, y.GetLogEntry())
				continue
			}
			if err = emit(y.GetLogEntry()); err != nil {
				return
			}
		}
	}
	if len(yy) > 0 {
		sort.Slice(yy, func(i, j int) bool {
			return yy[i].Time.Before(yy[j].Time)
		})
		for _, y := range yy {
			if err = emit(y); err != nil {
				return
			}
		}
	}
	return
}

// A LogEntry returns a node.LogEntry that should be logged. The method name
// GetLogEntry is non-idiomatic so that node.LogEntry may be embedded in
// implementations.
type LogEntry interface {
	GetLogEntry() node.LogEntry
}
