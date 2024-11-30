// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package antler

import (
	"embed"
	"io"
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/heistp/antler/node"
)

//go:embed node/bin/*
var nodeBin embed.FS

// nodeBinDir
const nodeBinDir = "node/bin"

// openNodeExe opens an embedded node executable for the given platform.
func openNodeExe(platform string) (fs.File, error) {
	n := node.PlatformExeName(platform)
	return nodeBin.Open(filepath.Join(nodeBinDir, n.String()))
}

// exeSource provides a node.ExeSource implementation for antler.
type exeSource struct {
}

// Reader implements ExeSource
func (e *exeSource) Reader(platform string) (io.ReadCloser, error) {
	return openNodeExe(platform)
}

// Size implements ExeSource
func (e *exeSource) Size(platform string) (size int64, err error) {
	var f fs.File
	if f, err = openNodeExe(platform); err != nil {
		return
	}
	var i fs.FileInfo
	if i, err = f.Stat(); err != nil {
		return
	}
	size = i.Size()
	return
}

// Platforms implements ExeSource
func (e *exeSource) Platforms() (platforms []string, err error) {
	var d []fs.DirEntry
	if d, err = nodeBin.ReadDir(nodeBinDir); err != nil {
		return
	}
	for _, e := range d {
		n := node.ExeName(e.Name())
		platforms = append(platforms, n.Platform())
	}
	sort.Strings(platforms)
	return
}
