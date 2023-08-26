// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"fmt"
	"strings"
	"time"
)

// logTimeFormat is the time format used for logging.
const logTimeFormat = "2006-01-02 15:04:05.000000"

// LogEntry represents one log entry.
type LogEntry struct {
	Time   time.Time // the time the entry was logged, per the node's clock
	NodeID NodeID    // the ID of the node that created the entry
	Tag    string    // tags the entry for categorization
	Text   string    // the entry's text
}

// init registers LogEntry with the gob encoder
func init() {
	gob.Register(LogEntry{})
}

// GetLogEntry implements antler.LogEntry
func (l LogEntry) GetLogEntry() LogEntry {
	return l
}

// flags implements message
func (LogEntry) flags() flag {
	return flagForward
}

// handle implements event
func (l LogEntry) handle(node *node) {
	node.parent.Send(l)
}

func (l LogEntry) String() string {
	t := l.Text
	if strings.Contains(t, "\n") {
		t = "⏎\n" + t
	}
	return fmt.Sprintf("%s %s %s: %s", l.Time.Format(logTimeFormat),
		l.NodeID, l.Tag, t)
}
