// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"errors"
	"fmt"
	"time"
)

// Error represents an unrecoverable error that occurred on a node.
type Error struct {
	LogEntry
}

// GetLogEntry implements antler.LogEntry
func (e Error) GetLogEntry() LogEntry {
	return e.LogEntry
}

// init registers Error with the gob encoder
func init() {
	gob.Register(Error{})
}

// flags implements message
func (e Error) flags() flag {
	return flagPush
}

func (e Error) Error() string {
	return e.String()
}

// ErrorFactory provides methods to create and return Errors.
type ErrorFactory struct {
	nodeID NodeID // the Error's NodeID
	tag    string // the Error's Tag
}

// NewError returns a new Error with the given message.
func (f ErrorFactory) NewError(message string) Error {
	t := time.Now()
	return Error{LogEntry{t, f.nodeID, f.tag, message}}
}

// NewErrore returns an Error from the given error. If the given error is
// already an Error, the existing error is returned.
func (f ErrorFactory) NewErrore(err error) Error {
	t := time.Now()
	if e, ok := err.(Error); ok {
		return e
	}
	return Error{LogEntry{t, f.nodeID, f.tag, err.Error()}}
}

// NewErrorf returns an Error with its Message formatted with prinf style args.
func (f ErrorFactory) NewErrorf(format string, a ...any) Error {
	t := time.Now()
	return Error{LogEntry{t, f.nodeID, f.tag, fmt.Sprintf(format, a...)}}
}

// errDone is an internal error sent on error channels to indicate the
// completion of a goroutine.
var errDone = errors.New("done")
