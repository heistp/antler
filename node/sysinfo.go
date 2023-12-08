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
	// Command lists the system commands to run.
	Command []SysInfoCommand

	// File lists the files to read.
	File []SysInfoFile

	// Env lists regex patterns of environment variables to retrieve.
	Env []string

	// Sysctl lists regex pattern of sysctl parameters to retrieve.
	Sysctl []string
}

// Run implements runner
func (s SysInfo) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	arg.rec.Logf("gathering system information")
	d := newSysInfoData(arg.rec.nodeID)
	if err = d.gather(s); err != nil {
		return
	}
	arg.rec.Send(d)
	return
}

// SysInfoData is a data object containing system information.
type SysInfoData struct {
	NodeID    ID                       // the ID of the Node the data comes from
	Hostname  string                   // hostname from os.Hostname()
	GoVersion string                   // Go version from runtime.Version()
	GoOS      string                   // Go OS from runtime.GOOS
	GoArch    string                   // Go Arch from runtime.GOARCH
	NumCPU    int                      // number of CPUs from runtime.NumCPU()
	Command   map[string]CommandOutput // map of command key to output
	File      map[string]FileData      // map of file key to data
	Env       map[string]string        // map of environment var name to value
	Sysctl    map[string]string        // map of sysctl params name to value
}

// CommandOutput contains the result of executing a command.
type CommandOutput struct {
	Out    []byte // the combined output from the command
	String string // the command string per Cmd.String()
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
func (s *SysInfoData) gather(info SysInfo) (err error) {
	// Go info
	if s.Hostname, err = os.Hostname(); err != nil {
		return
	}
	s.GoVersion = runtime.Version()
	s.GoOS = runtime.GOOS
	s.GoArch = runtime.GOARCH
	s.NumCPU = runtime.NumCPU()

	// commands
	for _, c := range info.Command {
		m := c.Cmd()
		var o []byte
		if o, err = m.CombinedOutput(); err != nil {
			err = fmt.Errorf("%s: %w\n%s", m.String(), err,
				strings.TrimSpace(string(o)))
			return
		}
		s.Command[c.key()] = CommandOutput{o, m.String()}
	}

	// files
	for _, f := range info.File {
		var d []byte
		if d, err = f.Read(); err != nil {
			return
		}
		s.File[f.key()] = FileData{f.Name, d}
	}

	// environment variables
	if len(info.Env) > 0 {
		var xx []*regexp.Regexp
		for _, s := range info.Env {
			var x *regexp.Regexp
			if x, err = regexp.Compile(s); err != nil {
				return
			}
			xx = append(xx, x)
		}
		for _, v := range os.Environ() {
			f := strings.SplitN(v, "=", 2)
			for _, x := range xx {
				if x.MatchString(f[0]) {
					s.Env[f[0]] = f[1]
					break
				}
			}
		}
	}

	// sysctls
	if len(info.Sysctl) > 0 {
		var xx []*regexp.Regexp
		for _, s := range info.Sysctl {
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
					s.Sysctl[k] = v
					break
				}
			}
		}
	}

	return
}

// Command represents the information needed to run a system command.
type Command struct {
	// Command is the command to run. The string is split into command name and
	// arguments using space as a delimiter, with no support for escaping. If
	// spaces are needed in arguments, use the Arg field instead, or in
	// addition to Command.
	Command string

	// Arg is a slice of arguments for the command. If Command is empty, then
	// Arg[0] is the command name, otherwise the Arg slice is appended to the
	// slice obtained by splitting Command.
	Arg []string
}

// Cmd returns an exec.Cmd with name and arg obtained from param().
func (c Command) Cmd() *exec.Cmd {
	n, a := c.Param()
	return exec.Command(n, a...)
}

// CmdContext returns an exec.Command using exec.CommandContext, with name and
// arg obtained from param().
func (c Command) CmdContext(ctx context.Context) *exec.Cmd {
	n, a := c.Param()
	return exec.CommandContext(ctx, n, a...)
}

// Name returns the command name.
func (c Command) Name() (name string) {
	name, _ = c.Param()
	return
}

// Param returns the name and arg parameters for exec.
func (c Command) Param() (name string, arg []string) {
	a := strings.Fields(c.Command)
	a = append(a, c.Arg...)
	name = a[0]
	arg = a[1:]
	return
}

// SysInfoCommand contains the info needed to execute a system command and
// return its output in SysInfoData.
type SysInfoCommand struct {
	Command
	Key string
}

// key returns the SysInfoData.Command map key.
func (c SysInfoCommand) key() string {
	if c.Key != "" {
		return c.Key
	}
	return c.Name()
}

// SysInfoFile contains the info needed to read a file and return its data in
// SysInfoData.
type SysInfoFile struct {
	Name string
	Key  string
}

// Read reads the file and returns the data.
func (f SysInfoFile) Read() (data []byte, err error) {
	data, err = os.ReadFile(f.Name)
	return
}

// key returns the SysInfoData.File map key.
func (f SysInfoFile) key() string {
	if f.Key != "" {
		return f.Key
	}
	return f.Name
}
