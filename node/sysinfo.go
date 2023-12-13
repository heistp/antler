// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package node

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// SysInfo gathers system information.
type SysInfo struct {
	// OS is a source that returns the operating system name / version.
	OS Texters

	// KernSrcInfo returns info on the kernel source code.
	KernSrcInfo Texters

	// KernSrcVer returns a version number or tag for the kernel source code.
	KernSrcVer Texters

	// Command lists the system commands to run.
	Command []Command

	// File lists the files to read.
	File []File

	// Env lists regex patterns of environment variables to retrieve.
	Env EnvVars

	// Sysctl lists regex pattern of sysctl parameters to retrieve.
	Sysctl Sysctls
}

// Run implements runner
func (s SysInfo) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	arg.rec.Logf("gathering system information")
	d := newSysInfoData(arg.rec.nodeID)
	if err = d.gather(ctx, s); err != nil {
		return
	}
	arg.rec.Send(d)
	return
}

// SysInfoData is a data object containing system information.
type SysInfoData struct {
	NodeID      ID                       // the ID of the Node the data comes from
	Hostname    string                   // hostname from os.Hostname()
	GoVersion   string                   // Go version from runtime.Version()
	GoOS        string                   // Go OS from runtime.GOOS
	GoArch      string                   // Go Arch from runtime.GOARCH
	NumCPU      int                      // number of CPUs from runtime.NumCPU()
	OS          string                   // OS name / version
	KernSrcInfo string                   // kernel source info
	KernSrcVer  string                   // kernel source version
	Command     map[string]CommandOutput // map of command key to output
	File        map[string]FileData      // map of file key to data
	Env         map[string]string        // map of environment var name to value
	Sysctl      map[string]string        // map of sysctl params name to value
}

// CommandOutput contains the result of executing a command.
type CommandOutput struct {
	Out    []byte // the combined output from the command
	String string // the command string per Cmd.String()
}

// Trim returns Out as a string, with whitespace trimmed.
func (c CommandOutput) Trim() string {
	return strings.TrimSpace(string(c.Out))
}

// newSysInfoData returns a new SysInfoData.
func newSysInfoData(nodeID ID) SysInfoData {
	return SysInfoData{
		NodeID:  nodeID,
		Command: make(map[string]CommandOutput),
		File:    make(map[string]FileData),
		Env:     make(map[string]string),
		Sysctl:  make(map[string]string),
	}
}

// init registers SysInfoData with the gob encoder
func init() {
	gob.Register(SysInfoData{})
}

// flags implements message
func (SysInfoData) flags() flag {
	return flagForward
}

// handle implements event
func (s SysInfoData) handle(node *node) {
	node.parent.Send(s)
}

// gather collects all SysInfoData fields from the system.
func (s *SysInfoData) gather(ctx context.Context, info SysInfo) (err error) {
	// Go info
	if s.Hostname, err = os.Hostname(); err != nil {
		return
	}
	s.GoVersion = runtime.Version()
	s.GoOS = runtime.GOOS
	s.GoArch = runtime.GOARCH
	s.NumCPU = runtime.NumCPU()

	// fixed fields
	if t := info.OS.texter(); t != nil {
		if s.OS, err = t.Text(ctx); err != nil {
			return
		}
	}
	if t := info.KernSrcInfo.texter(); t != nil {
		if s.KernSrcInfo, err = t.Text(ctx); err != nil {
			return
		}
	}
	if t := info.KernSrcVer.texter(); t != nil {
		if s.KernSrcVer, err = t.Text(ctx); err != nil {
			return
		}
	}

	// commands
	for _, c := range info.Command {
		m := c.CmdContext(ctx)
		var o []byte
		if o, err = m.CombinedOutput(); err != nil {
			err = CommandError{err, m.String(), o}
			return
		}
		s.Command[m.String()] = CommandOutput{o, m.String()}
	}

	// files
	for _, f := range info.File {
		var d []byte
		if d, err = f.Data(); err != nil {
			return
		}
		s.File[f.Name()] = FileData{f.Name(), d}
	}

	// environment variables
	if err = info.Env.get(s.Env); err != nil {
		return
	}

	// sysctls
	if err = info.Sysctl.get(s.Sysctl); err != nil {
		return
	}

	return
}

// A Texter can return a string from a source that may return an error.
type Texter interface {
	Text(context.Context) (string, error)
}

// Texters is a union of the available Texter implementations. Only one of the
// fields may be non-nil.
type Texters struct {
	Command *Command
	File    *File
	Env     *EnvVar
	Sysctl  *Sysctl
}

// texter returns the only non-nil Texter implementation.
func (t *Texters) texter() Texter {
	switch {
	case t.Command != nil:
		return t.Command
	case t.File != nil:
		return t.File
	case t.Env != nil:
		return t.Env
	case t.Sysctl != nil:
		return t.Sysctl
	}
	return nil
}

// File represents a file name, and implements Texter to retrieve its data as
// text.
type File string

// Text implements Texter
func (f File) Text(ctx context.Context) (txt string, err error) {
	var d []byte
	if d, err = f.Data(); err != nil {
		return
	}
	txt = strings.TrimSpace(string(d))
	return
}

// Name returns the file name.
func (f File) Name() string {
	return string(f)
}

// Data returns the file data as a byte slice.
func (f File) Data() (data []byte, err error) {
	data, err = os.ReadFile(f.Name())
	return
}

// EnvVar represents the name of a single environment variable.
type EnvVar string

// Text implements Texter
func (v EnvVar) Text(ctx context.Context) (txt string, err error) {
	for _, v := range os.Environ() {
		f := strings.SplitN(v, "=", 2)
		if f[0] == v {
			txt = f[1]
			return
		}
	}
	err = fmt.Errorf("environment variable not set: %s", v)
	return
}

// EnvVars represents a list of patterns of environment variables to retrieve.
type EnvVars []string

// get puts the matching environment variables into the given map.
func (v EnvVars) get(vars map[string]string) (err error) {
	var xx []*regexp.Regexp
	for _, s := range v {
		var x *regexp.Regexp
		if x, err = regexp.Compile(s); err != nil {
			return
		}
		xx = append(xx, x)
	}
	for _, l := range os.Environ() {
		f := strings.SplitN(l, "=", 2)
		for _, x := range xx {
			if x.MatchString(f[0]) {
				vars[f[0]] = f[1]
				break
			}
		}
	}
	return
}

// Sysctl represents the key of a sysctl kernel parameter.
type Sysctl string

// Text implements Texter
func (s Sysctl) Text(ctx context.Context) (txt string, err error) {
	var o []byte
	if o, err = exec.Command("sysctl", "-n", string(s)).Output(); err != nil {
		return
	}
	txt = strings.TrimSpace(string(o))
	return
}

// Sysctls represents a list of patterns of sysctls to retrieve.
type Sysctls []string

func (y Sysctls) get(params map[string]string) (err error) {
	if len(y) == 0 {
		return
	}
	var xx []*regexp.Regexp
	for _, s := range y {
		var x *regexp.Regexp
		if x, err = regexp.Compile(s); err != nil {
			return
		}
		xx = append(xx, x)
	}
	var o []byte
	if o, err = exec.Command("sysctl", "-a").Output(); err != nil {
		return
	}
	a := bufio.NewScanner(bytes.NewReader(o))
	for a.Scan() {
		f := strings.SplitN(a.Text(), "=", 2)
		k := strings.TrimSpace(f[0])
		v := strings.TrimSpace(f[1])
		for _, x := range xx {
			if x.MatchString(k) {
				params[k] = v
				break
			}
		}
	}
	return
}
