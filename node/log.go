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

// init registers LogEntry with the gob encoder
func init() {
	gob.Register(LogEntry{})
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
		t = "->\n" + t
	}
	return fmt.Sprintf("%s %s %s: %s", e.Time.Format(readableTimeFormat),
		e.NodeID, e.Tag, t)
}
