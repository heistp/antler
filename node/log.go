// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"fmt"
	"strings"
	"time"
)

// LogEntry represents one log entry.
type LogEntry struct {
	Time   time.Time // the time of the LogEntry, per the node's clock
	NodeID string    // the ID of the node that created the LogEntry
	Tag    string    // tags the LogEntry for categorization
	Text   string    // the text message for the LogEntry
}

func init() {
	gob.Register(LogEntry{})
}

// DataPoint implements DataPointer
func (l LogEntry) DataPoint() DataPoint {
	b := strings.Builder{}
	b.Grow(len(l.NodeID) + 5 + len(l.Tag))
	b.WriteString(l.NodeID)
	b.WriteString(".log.")
	b.WriteString(l.Tag)
	s := Series(b.String())
	return DataPoint{s, Time{l.Time}, l.Text}
}

// flags implements message
func (LogEntry) flags() flag {
	return flagForward
}

// handle implements event
func (l LogEntry) handle(node *node) {
	node.parent.Send(l)
}

// String returns the entry for display.
func (e LogEntry) String() string {
	t := e.Text
	if strings.Contains(t, "\n") {
		t = "\n" + t
	}
	return fmt.Sprintf("%s %s %s: %s", e.Time.Format(readableTimeFormat),
		e.NodeID, e.Tag, t)
}
