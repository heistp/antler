// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"time"
)

// recorder is a helper used for logging, recording data points and creating
// Error's. recorder must be created using newRecorder, and is safe for
// concurrent use.
type recorder struct {
	nodeID ID
	tag    string
	parent *conn
	ErrorFactory
}

// newRecorder returns a new recorder.
func newRecorder(nodeID ID, tag string, parent *conn) *recorder {
	return &recorder{
		nodeID,
		tag,
		parent,
		ErrorFactory{nodeID, tag},
	}
}

// WithTag returns a copy of this recorder replacing tag with the given tag.
func (r *recorder) WithTag(tag string) *recorder {
	return &recorder{
		r.nodeID,
		tag,
		r.parent,
		ErrorFactory{r.nodeID, tag},
	}
}

// Logf sends a LogEntry using printf style args.
func (r *recorder) Logf(format string, a ...any) {
	t := time.Now()
	m := fmt.Sprintf(format, a...)
	r.Send(LogEntry{t, r.nodeID, r.tag, m})
}

// Log sends a LogEntry with the given message.
func (r *recorder) Log(message string) {
	t := time.Now()
	r.Send(LogEntry{t, r.nodeID, r.tag, message})
}

// FileData sends a FileData.
func (r *recorder) FileData(name string, data []byte) {
	r.Send(FileData{name, data})
}

// Stream sends a Stream filter to the parent conn.
func (r *recorder) Stream(s *ResultStream) {
	r.parent.Stream(s)
}

// SendError sends an Error created by NewError.
func (r *recorder) SendError(message string) {
	r.Send(r.NewError(message))
}

// SendErrore sends an error created by NewErrore.
func (r *recorder) SendErrore(err error) {
	r.Send(r.NewErrore(err))
}

// SendErrorf sends an error created by NewErrorf.
func (r *recorder) SendErrorf(format string, a ...any) {
	r.Send(r.NewErrorf(format, a...))
}

// Send sends a message to the parent conn.
func (r *recorder) Send(msg message) {
	r.parent.Send(msg)
}

// logFunc is called to log a message with the given format and text.
type logFunc func(format string, a ...any)
