// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// System is a runner that executes a system command.
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
	// (false). If errors are discarded, they will still be logged, but an error
	// will not be returned, so the Run tree may continue.
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

	cmd     *exec.Cmd
	outw    sync.WaitGroup
	gatherc chan string
	gathern int
}

// Run implements runner
func (s *System) Run(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler) (ofb Feedback, err error) {
	defer func() {
		if s.IgnoreErrors {
			if err != nil {
				rec.Logf("%s", err)
			}
			err = nil
		}
	}()
	n, a := s.params()
	c := exec.CommandContext(ctx, n, a...)
	rec.Logf("%s", c)
	if err = s.handleOutput(s.Stdout, c.StdoutPipe, rec); err != nil {
		return
	}
	if err = s.handleOutput(s.Stderr, c.StderrPipe, rec); err != nil {
		return
	}
	if err = c.Start(); err != nil {
		return
	}
	if s.Background {
		s.cmd = c
		cxl <- s
		return
	}
	err = c.Wait()
	s.outw.Wait()
	return
}

// Cancel implements canceler
func (s *System) Cancel(rec *recorder) (err error) {
	if err = s.cmd.Wait(); err != nil {
		rec.Logf("%s", err)
		err = nil
	}
	s.outw.Wait()
	return
}

// handleOutput is called to start processing of stdout and stderr.
func (s *System) handleOutput(treatment string, pipe pipeFunc,
	rec *recorder) (err error) {
	if treatment != "quiet" {
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
			err = fmt.Errorf("Stdout/Stderr files not supported")
		}
	}
	return
}

// pipeFunc defines a function for StdoutPipe and StderrPipe from exec.Cmd.
type pipeFunc func() (io.ReadCloser, error)

// gatherDone is a magic string indicating a gather goroutine is done.
const gatherDone = "cf799836-40d7-488d-9a87-a8bf5c92691b"

// gather contains goroutines that gather lines from rcl, and log them after
// completion.
func (s *System) gather(rcl io.ReadCloser, rec *recorder) {
	s.gathern++
	go func() {
		a := bufio.NewScanner(rcl)
		for a.Scan() {
			s.gatherc <- a.Text()
		}
		s.gatherc <- gatherDone
	}()
	if s.gatherc != nil {
		return
	}
	s.gatherc = make(chan string)
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

// params returns the name and args parameters for exec.
func (s *System) params() (name string, args []string) {
	a := strings.Fields(s.Command)
	a = append(a, s.Args...)
	name = a[0]
	args = a[1:]
	return
}
