// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"path/filepath"
)

// Stream selects messages for either streaming or buffering.
type Stream struct {
	// Include accepts messages to stream.
	Include *StreamFilter

	// Exclude rejects messages to stream, and buffers them instead.
	Exclude *StreamFilter
}

// Run implements runner
func (s *Stream) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	if s.Include != nil {
		if err = s.Include.validate(); err != nil {
			return
		}
	}
	if s.Exclude != nil {
		if err = s.Exclude.validate(); err != nil {
			return
		}
	}
	arg.rec.Stream(s)
	return
}

// accept returns true if the given message should be streamed.
func (s *Stream) accept(msg message) (stream bool) {
	if s.Include != nil {
		if stream = s.Include.accept(msg); !stream {
			return
		}
	}
	if s.Exclude != nil {
		var x bool
		if x = s.Exclude.accept(msg); x {
			return
		}
	}
	return
}

// StreamFilter selects messages for either streaming or buffering.
type StreamFilter struct {
	// File is a valid glob pattern of FileData names to accept.
	File []string

	// Log indicates whether to accept (true) or reject (false) LogEntry's.
	Log bool

	// Series to accept.
	Series []Series
}

// accept returns true if the StreamFilter accepts the given message.
func (f *StreamFilter) accept(msg message) (verdict bool) {
	switch v := msg.(type) {
	case FileData:
		for _, p := range f.File {
			if verdict, _ = filepath.Match(p, v.Name); verdict {
				return
			}
		}
	case LogEntry:
		verdict = f.Log
		return
	case DataPoint:
		for _, s := range f.Series {
			if v.Series == s {
				verdict = true
				return
			}
		}
	}
	return
}

// validate returns an error if the StreamFilter is invalid.
func (f *StreamFilter) validate() (err error) {
	for _, p := range f.File {
		if _, err = filepath.Match(p, ""); err != nil {
			return
		}
	}
	return
}
