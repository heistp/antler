// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"fmt"
	"strings"
	"time"
)

// FileData contains a chunk of binary data to be saved in a file.
type FileData struct {
	Time   time.Time // the time the FileData was created, per the node's clock
	NodeID string    // the ID of the node that created the FileData
	Name   string    // the name of the file
	Data   []byte    // the data
}

// init registers FileData with the gob encoder
func init() {
	gob.Register(FileData{})
}

// fileDataTag is used when creating the DataPoint Series for FileData's.
const fileDataTag = ".file."

// DataPoint implements DataPointer
func (f FileData) DataPoint() DataPoint {
	b := strings.Builder{}
	b.Grow(len(f.NodeID) + len(fileDataTag) + len(f.Name))
	b.WriteString(f.NodeID)
	b.WriteString(fileDataTag)
	b.WriteString(f.Name)
	s := Series(b.String())
	return DataPoint{s, Time{f.Time}, f.Data}
}

// flags implements message
func (FileData) flags() flag {
	return flagForward
}

// handle implements event
func (f FileData) handle(node *node) {
	node.parent.Send(f)
}

func (f FileData) String() string {
	return fmt.Sprintf("FileData[NodeID:%s Name:%s Len:%d]", f.NodeID, f.Name,
		len(f.Data))
}
