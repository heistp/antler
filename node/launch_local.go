// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package node

import (
	"os/exec"
	"strings"
	"syscall"
)

// Local is a launcher used to start a node as a locally executed process.
type Local struct {
	Sudo bool
	Set  bool
}

// launch implements launcher
func (l Local) launch(node Node, log logFunc) (tr transport, err error) {
	cl := newCloserStack(log)
	defer func() {
		if err != nil {
			cl.Close()
		}
	}()
	var f *exeFile
	if f, err = repo.File(node.Platform); err != nil {
		return
	}
	cl.Push(f)
	ns := node.Netns.Name
	if node.Netns.Create {
		if ns == "" {
			ns = string(node.ID)
		}
		if err = addNetns(ns, log); err != nil {
			return
		}
		cl.Push(deleteNetns{ns})
	}
	var a []string
	if l.Sudo {
		a = append(a, "sudo")
	}
	if ns != "" {
		a = append(a, "ip")
		a = append(a, "netns")
		a = append(a, "exec")
		a = append(a, ns)
	} else {
	}
	a = append(a, f.Path)
	a = append(a, string(node.ID))
	c := exec.Command(a[0], a[1:]...)
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	c.Env = node.Env.vars()
	log("%s", c)
	var nc *nodeCmd
	if nc, err = newNodeCmd(c, cl, log); err != nil {
		return
	}
	if err = nc.Start(); err != nil {
		return
	}
	tr = newGobTransport(nc)
	return
}

/*
// alphaNum is the set of alphanumeric characters.
const alphaNum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randString returns a pseudo-random alphanumeric string of the given length.
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphaNum[rand.Intn(len(alphaNum))]
	}
	return string(b)
}
*/

// addNetns adds a network namespace.
func addNetns(name string, log logFunc) (err error) {
	c := exec.Command("ip", "netns", "add", name)
	log("%s", c.String())
	var out []byte
	out, err = c.CombinedOutput()
	if len(out) > 0 {
		log("%s", strings.TrimSpace(string(out)))
	}
	return
}

// closerStack receives and pushes closers to a stack. When Close is called, the
// closers are popped from the stack and run.
type closerStack struct {
	cl   chan closer
	log  logFunc
	errc chan error
}

// newCloserStack returns a new instance of closerStack.
func newCloserStack(log logFunc) *closerStack {
	s := &closerStack{
		make(chan closer),
		log,
		make(chan error),
	}
	go s.run()
	return s
}

// Push pushes a closer to the stack.
func (s *closerStack) Push(cl closer) {
	s.cl <- cl
}

// Close implements io.Closer. It must be called exactly once.
func (s *closerStack) Close() (err error) {
	close(s.cl)
	for e := range s.errc {
		if e != nil {
			s.log("%s", e)
			if err == nil {
				err = e
			}
		}
	}
	return
}

// run gathers closers on the stack, and runs them when cl is closed. Any errors
// are sent to the given error channel, which is closed after completion.
func (s *closerStack) run() {
	defer close(s.errc)
	a := make([]closer, 0, 16)
	for c := range s.cl {
		a = append(a, c)
	}
	for i := len(a) - 1; i >= 0; i-- {
		c := a[i]
		if e := c.Close(s.log); e != nil {
			s.errc <- e
		}
	}
}

// A closer closes or cleans up after a launch.
type closer interface {
	Close(logFunc) error
}

// deleteNetns is a closer that deletes a network namespace.
type deleteNetns struct {
	name string
}

func (d deleteNetns) Close(log logFunc) (err error) {
	c := exec.Command("ip", "netns", "del", d.name)
	log("%s", c.String())
	var out []byte
	if out, err = c.CombinedOutput(); len(out) > 0 {
		log("%s", strings.TrimSpace(string(out)))
	}
	return
}
