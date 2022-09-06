// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// executor is used to run and log system commands.
type executor struct {
	log logFunc
}

// newExecutor returns a new executor.
func newExecutor(log logFunc) *executor {
	return &executor{log}
}

// Run executes the named command with the given arguments.
func (e *executor) Run(name string, arg ...string) error {
	return e.Runc(nil, name, arg...)
}

// Runc executes the named command with the given arguments. If the given
// Context is canceled, the command terminates.
func (e *executor) Runc(ctx context.Context, name string, arg ...string) (
	err error) {
	var c *exec.Cmd
	if ctx != nil {
		c = exec.CommandContext(ctx, name, arg...)
	} else {
		c = exec.Command(name, arg...)
	}
	e.log("%s", c)
	var o []byte
	o, err = c.CombinedOutput()
	if s := strings.TrimSpace(string(o)); len(s) > 0 {
		e.log("%s", s)
	}
	return
}

// Runs executes a command that may be safely split into multiple arguments by
// spaces.
func (e *executor) Runs(cmd string) error {
	return e.Runcs(nil, cmd)
}

// Runcs executes a command that may be safely split into multiple arguments by
// spaces. If the given Context is canceled, the command terminates.
func (e *executor) Runcs(ctx context.Context, cmd string) error {
	f := strings.Fields(cmd)
	return e.Runc(ctx, f[0], f[1:]...)
}

// Runf executes a command using the given printf style format string and
// arguments. It must be possible to safely split the resulting string by spaces
// into a command with multiple arguments.
func (e *executor) Runf(format string, arg ...interface{}) error {
	return e.Runs(fmt.Sprintf(format, arg...))
}

// Runcf executes a command using the given printf style format string and
// arguments. It must be possible to safely split the resulting string by spaces
// into a command with multiple arguments. If the given Context is canceled, the
// command terminates.
func (e *executor) Runcf(ctx context.Context, format string,
	arg ...interface{}) error {
	return e.Runcs(ctx, fmt.Sprintf(format, arg...))
}
