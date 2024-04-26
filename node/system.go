// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// System executes a system command.
type System struct {
	// Command is the embedded system command.
	Command

	// Background indicates whether to run this command in the background (true)
	// or foreground (false). If true, Run will return as soon as the command is
	// started, and with an error if it could not be started and IgnoreErrors is
	// false. The Context will be cancelled after the rest of the Run tree is
	// complete, at which time the process will be interrupted or killed
	// (according to Kill), and the node will wait for it to complete.
	Background bool

	// IgnoreErrors indicates whether to discard any errors (true) or not
	// (false).
	IgnoreErrors bool

	// Stdout selects the treatment for stdout. If empty, stdout is gathered and
	// emitted to the log as a single line when the command completes. If
	// "stream", stdout is emitted to the log a line at a time. If "quiet",
	// stdout is discarded. Otherwise, stdout is written to a file of the given
	// name.
	Stdout string

	// Stderr selects the treatment for stderr, with the same semantics as for
	// Stdout.
	Stderr string

	// Kill indicates whether to kill the process on cancellation (true) or
	// signal it with an interrupt (false).
	Kill bool

	io      sync.WaitGroup
	gatherC chan string
	gatherN int
}

// Run implements runner
func (s *System) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	if s.IgnoreErrors {
		defer func() {
			err = nil
		}()
	}
	c := s.CmdContext(ctx)
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w (%s)", err, c)
		}
	}()
	if !s.Kill {
		c.Cancel = func() error {
			return c.Process.Signal(os.Interrupt)
		}
		c.WaitDelay = 1 * time.Second
	}
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	arg.rec.Logf("%s", c)
	if err = s.handleOutput(s.Stdout, c.StdoutPipe, arg.rec); err != nil {
		return
	}
	if err = s.handleOutput(s.Stderr, c.StderrPipe, arg.rec); err != nil {
		return
	}
	if s.gatherN > 0 {
		s.gatherLog(arg.rec)
	}
	if err = c.Start(); err != nil {
		return
	}
	var x cancelFunc = func() error {
		s.io.Wait()
		e := c.Wait()
		if s.Background {
			if e != nil {
				if errors.Is(e, context.Canceled) {
					arg.rec.Logf("exited: %s", c)
				} else {
					arg.rec.Logf("background error: %s (%s)", e, c)
				}
			}
			return nil
		}
		return e
	}
	if s.Background {
		arg.cxl <- x
		return
	}
	err = x()
	return
}

// handleOutput is called to start processing of stdout and stderr.
func (s *System) handleOutput(treatment string, pipe pipeFunc,
	rec *recorder) (err error) {
	if treatment == "quiet" {
		return
	}
	var r io.ReadCloser
	if r, err = pipe(); err != nil {
		return
	}
	switch treatment {
	case "":
		fallthrough
	case "gather":
		s.gather(r, rec)
	case "stream":
		s.stream(r, rec)
	default:
		s.file(r, treatment, rec)
	}
	return
}

// pipeFunc defines a function for StdoutPipe and StderrPipe from exec.Cmd.
type pipeFunc func() (io.ReadCloser, error)

// gatherDone is a magic string indicating a gather goroutine is done.
const gatherDone = "cf799836-40d7-488d-9a87-a8bf5c92691b"

// gather contains a goroutine to read lines from rcl and send them to gatherC.
func (s *System) gather(rcl io.ReadCloser, rec *recorder) {
	s.gatherN++
	if s.gatherC == nil {
		s.gatherC = make(chan string)
	}
	go func() {
		defer func() {
			s.gatherC <- gatherDone
		}()
		a := bufio.NewScanner(rcl)
		for a.Scan() {
			s.gatherC <- a.Text()
		}
	}()
}

// gatherLog contains a goroutine to read lines from gatherC, and log them with
// one call when once gatherN reaches zero.
func (s *System) gatherLog(rec *recorder) {
	s.io.Add(1)
	go func() {
		defer s.io.Done()
		var b bytes.Buffer
		for l := range s.gatherC {
			if l == "" {
				continue
			}
			if l == gatherDone {
				s.gatherN--
				if s.gatherN == 0 {
					break
				}
				continue
			}
			fmt.Fprintln(&b, l)
		}
		o := strings.TrimSpace(b.String())
		if o == "" {
			return
		}
		rec.Logf("%s", o)
	}()
}

// stream contains a goroutine to log the given ReadCloser, a line at a time.
func (s *System) stream(rcl io.ReadCloser, rec *recorder) {
	s.io.Add(1)
	go func() {
		defer s.io.Done()
		c := bufio.NewScanner(rcl)
		for c.Scan() {
			rec.Logf("%s", c.Text())
		}
	}()
}

// file contains a goroutine to send data from the given ReadCloser as FileData.
func (s *System) file(rcl io.ReadCloser, name string, rec *recorder) {
	s.io.Add(1)
	go func() {
		defer s.io.Done()
		var e error
		for {
			b := make([]byte, 64*1024)
			var n int
			n, e = rcl.Read(b)
			if n > 0 {
				rec.FileData(name, b[:n])
			}
			if e != nil {
				if e != io.EOF {
					rec.Logf("%s", e)
				}
				break
			}
		}
	}()
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
	n, a := c.param()
	return exec.Command(n, a...)
}

// CmdContext returns an exec.Command using exec.CommandContext, with name and
// arg obtained from param().
func (c Command) CmdContext(ctx context.Context) *exec.Cmd {
	n, a := c.param()
	return exec.CommandContext(ctx, n, a...)
}

// Text implements Texter
func (c Command) Text(ctx context.Context) (txt string, err error) {
	m := c.CmdContext(ctx)
	var o []byte
	if o, err = m.CombinedOutput(); err != nil {
		err = CommandError{err, m.String(), o}
		return
	}
	txt = strings.TrimSpace(string(o))
	return
}

// param returns the name and arg parameters for exec.
func (c Command) param() (name string, arg []string) {
	a := strings.Fields(c.Command)
	a = append(a, c.Arg...)
	name = a[0]
	arg = a[1:]
	return
}

// CommandError wraps an error after executing a command to provide more
// detailed output.
type CommandError struct {
	Err     error
	Command string
	Out     []byte
}

func (c CommandError) Error() string {
	return fmt.Sprintf("%s: %s\n%s", c.Command, c.Err,
		strings.TrimSpace(string(c.Out)))
}

// Unwrap returns the inner error, for errors.Is/As.
func (c CommandError) Unwrap() error {
	return c.Err
}
