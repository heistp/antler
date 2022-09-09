// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"fmt"
)

// FileData contains a chunk of binary data to be saved in a file.
type FileData struct {
	NodeID string // the ID of the node that created the FileData
	Name   string // the name of the file
	Data   []byte // the data
}

// init registers FileData with the gob encoder
func init() {
	gob.Register(FileData{})
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
