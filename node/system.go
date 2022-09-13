// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// System executes a system command.
type System struct {
	// Command is the command to run. The string is split into command name and
	// arguments using space as a delimiter, with no support for escaping. If
	// spaces are needed in arguments, use the Args field instead, or in
	// addition to Command.
	Command string

	// Args is a slice of arguments for the command. If Command is empty, then
	// Args[0] is the command name, otherwise the Args slice is appended to the
	// slice obtained by splitting Command.
	Args []string

	// Background indicates whether to run this command in the background (true)
	// or foreground (false). If true, Run will return as soon as the command is
	// started, and with an error if it could not be started and IgnoreErrors is
	// false. The Context will be cancelled after the rest of the Run tree is
	// complete, at which time the process will be killed, and the node will
	// wait for it to complete.
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

	cmd      *exec.Cmd
	outw     sync.WaitGroup
	gatherc  chan string
	gathern  int
	procDone chan struct{}
}

// Run implements runner
func (s *System) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	if s.IgnoreErrors {
		defer func() {
			err = nil
		}()
	}
	n, a := s.params()
	var c *exec.Cmd
	if s.Kill {
		c = exec.CommandContext(ctx, n, a...)
	} else {
		c = exec.Command(n, a...)
	}
	arg.rec.Logf("%s", c)
	if err = s.handleOutput(s.Stdout, c.StdoutPipe, arg.rec); err != nil {
		return
	}
	if err = s.handleOutput(s.Stderr, c.StderrPipe, arg.rec); err != nil {
		return
	}
	if s.gathern > 0 {
		s.gatherLog(arg.rec)
	}
	if err = c.Start(); err != nil {
		return
	}
	s.procDone = make(chan struct{})
	if !s.Kill {
		s.interrupt(ctx, c.Process)
	}
	if s.Background {
		s.cmd = c
		arg.cxl <- s
		return
	}
	err = c.Wait()
	close(s.procDone)
	s.outw.Wait()
	return
}

// Cancel implements canceler
func (s *System) Cancel(rec *recorder) (err error) {
	if err = s.cmd.Wait(); err != nil {
		rec.Logf("%s", err)
		err = nil
	}
	close(s.procDone)
	s.outw.Wait()
	return
}

// interrupt starts a goroutine to interrupt the started process after the
// Context is canceled, then kill it if it hasn't completed after 2 seconds.
func (s *System) interrupt(ctx context.Context, proc *os.Process) {
	go func() {
		select {
		case <-ctx.Done():
			go func() {
				select {
				case <-time.After(2 * time.Second):
					proc.Kill()
				case <-s.procDone:
				}
			}()
			// NOTE this should not attempt to interrupt on Windows
			proc.Signal(os.Interrupt)
		case <-s.procDone:
		}
	}()
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

// gather contains a goroutine to read lines from rcl and send them to gatherc.
func (s *System) gather(rcl io.ReadCloser, rec *recorder) {
	s.gathern++
	if s.gatherc == nil {
		s.gatherc = make(chan string)
	}
	go func() {
		defer func() {
			s.gatherc <- gatherDone
		}()
		a := bufio.NewScanner(rcl)
		for a.Scan() {
			s.gatherc <- a.Text()
		}
	}()
}

// gatherLog contains a goroutine to read lines from gatherc, and log them with
// one call when once gathern reaches zero.
func (s *System) gatherLog(rec *recorder) {
	s.outw.Add(1)
	go func() {
		defer s.outw.Done()
		var b bytes.Buffer
		for l := range s.gatherc {
			if l == "" {
				continue
			}
			if l == gatherDone {
				s.gathern--
				if s.gathern == 0 {
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
	s.outw.Add(1)
	go func() {
		defer s.outw.Done()
		c := bufio.NewScanner(rcl)
		for c.Scan() {
			rec.Logf("%s", c.Text())
		}
	}()
}

// file contains a goroutine to send data from the given ReadCloser as FileData.
func (s *System) file(rcl io.ReadCloser, name string, rec *recorder) {
	s.outw.Add(1)
	go func() {
		defer s.outw.Done()
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

// params returns the name and args parameters for exec.
func (s *System) params() (name string, args []string) {
	a := strings.Fields(s.Command)
	a = append(a, s.Args...)
	name = a[0]
	args = a[1:]
	return
}
