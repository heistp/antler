// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

package antler

import "sync"

// errChans wraps a list of read-only error channels to make it more convenient
// to manage and merge errors.
type errChans []<-chan error

// add adds the given error channel.
func (e *errChans) add(c <-chan error) {
	*e = append(*e, c)
}

// make creates, adds and returns a new error channel.
func (e *errChans) make() (c chan error) {
	c = make(chan error)
	e.add(c)
	return
}

// merge reads and combines errors from the given channels into the returned out
// channel, which is closed after all the error channels are closed. It follows
// the contract of mergeErr.
func (e *errChans) merge() (out <-chan error) {
	return mergeErr(*e...)
}

// mergeErr confines goroutines to combine errors from the given in channels to
// the returned out channel. If any of the in channels are nil, they are not
// read and do not hold up the merge. The out channel is closed after all in
// channels are closed. mergeErr does not consume nil errors, so those are
// sent to out as well.
func mergeErr(in ...<-chan error) (out <-chan error) {
	oc := make(chan error)
	out = oc
	var wg sync.WaitGroup
	for _, c := range in {
		wg.Add(1)
		go func(c <-chan error) {
			defer wg.Done()
			if c == nil {
				return
			}
			for e := range c {
				oc <- e
			}
		}(c)
	}
	go func() {
		wg.Wait()
		close(oc)
	}()
	return
}
