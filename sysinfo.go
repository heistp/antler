// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

package antler

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/heistp/antler/node"
)

// sysinfoTemplate is the template for emitting system information from
// SysInfoData.
//
//go:embed sysinfo.html.tmpl
var sysinfoTemplate string

// EmitSysInfo is a reporter that emits SysInfoData's to files and/or stdout.
type EmitSysInfo struct {
	// To lists the destinations to send output to. "-" sends output to stdout,
	// and everything else sends output to the named file. If To is empty,
	// output is emitted to stdout. If two contains the verb %s, it is replaced
	// by the Node ID.
	To []string
}

// report implements reporter
func (y *EmitSysInfo) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	t := template.New("SysInfo")
	if t, err = t.Parse(sysinfoTemplate); err != nil {
		return
	}
	for d := range in {
		out <- d
		if i, ok := d.(node.SysInfoData); ok {
			if err = y.emit(rw, t, i); err != nil {
				return
			}
		}
	}
	return
}

// emit emits a single SysInfoData to all the destinations in To.
func (y *EmitSysInfo) emit(rw rwer, tpl *template.Template,
	info node.SysInfoData) (err error) {
	var ww []io.WriteCloser
	defer func() {
		for _, w := range ww {
			if e := w.Close(); e != nil && err == nil {
				err = e
			}
		}
	}()
	for _, s := range y.To {
		if strings.Contains(s, "%s") {
			s = fmt.Sprintf(s, info.NodeID)
		}
		ww = append(ww, rw.Writer(s))
	}
	err = tpl.Execute(multiWriteCloser(ww...), info)
	return
}
