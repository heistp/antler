// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"time"
)

// recorder is a helper used for logging, recording DataPoint's and creating
// Error's. recorder must be created using newRecorder, and is safe for
// concurrent use. Close must be called after use to stop the internal goroutine
// used for data buffering.
type recorder struct {
	nodeID string
	tag    string
	parent *conn
	ErrorFactory
}

// newRecorder returns a new recorder.
func newRecorder(nodeID, tag string, parent *conn) *recorder {
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
func (r *recorder) Logf(format string, a ...interface{}) {
	t := time.Now()
	m := fmt.Sprintf(format, a...)
	r.parent.Send(LogEntry{t, r.nodeID, r.tag, m})
}

// Log sends a LogEntry with the given message.
func (r *recorder) Log(message string) {
	t := time.Now()
	r.parent.Send(LogEntry{t, r.nodeID, r.tag, message})
}

// FileData sends a FileData.
func (r *recorder) FileData(name string, data []byte) {
	t := time.Now()
	r.parent.Send(FileData{t, r.nodeID, name, data})
}

// logFunc is called to log a message with the given format and text.
type logFunc func(format string, a ...interface{})
