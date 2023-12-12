// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"fmt"
	"strings"
)

// FileData contains a chunk of binary data to be saved in a file.
type FileData struct {
	Name string // the name of the file
	Data []byte // the data
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

// Trim returns Data as a string, with whitespace trimmed.
func (f FileData) Trim() string {
	return strings.TrimSpace(string(f.Data))
}

func (f FileData) String() string {
	return fmt.Sprintf("FileData[Name:%s Len:%d]", f.Name, len(f.Data))
}
