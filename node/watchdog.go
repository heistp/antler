// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import "time"

// watchdog keeps track of operations and signals timeouts when an operation has
// exceeded its deadline. watchdog must be created with newWatchdog, and stopped
// with Stop to release any resources. It is safe for concurrent use.
//
// TODO re-implement watchdog to use mutexes
type watchdog struct {
	watch chan timeout
	done  chan struct{}
}

func newWatchdog() watchdog {
	w := watchdog{
		make(chan timeout),
		make(chan struct{}),
	}
	go w.run()
	return w
}

// Watch sets a timeout for the given key for the given wait time from now. Upon
// timeout, a timeout value is sent to the notify channel using a non-blocking
// send, once per second until it is received.
func (w *watchdog) Watch(key interface{}, wait time.Duration,
	notify chan<- timeout) {
	w.watch <- timeout{key, wait, time.Now().Add(wait), notify}
}

// Unwatch removes the timeout with the given key.
func (w *watchdog) Unwatch(key interface{}) {
	w.watch <- timeout{key, 0, time.Time{}, nil}
}

// Stop stops the watchdog, waits for it to complete and releases any resources.
func (w *watchdog) Stop() {
	close(w.watch)
	<-w.done
}

// run is the watchdog goroutine's entry point.
func (w *watchdog) run() {
	defer close(w.done)
	tck := time.NewTicker(time.Second)
	defer tck.Stop()
	o := make(map[interface{}]timeout)
	for {
		select {
		case t, ok := <-w.watch:
			if !ok {
				break
			}
			if t.Wait == 0 {
				delete(o, t.Key)
				break
			}
			o[t.Key] = t
		case <-tck.C:
			n := time.Now()
			for k, t := range o {
				if t.Deadline.Before(n) {
					select {
					case t.Notify <- t:
						delete(o, k)
					default:
					}
				}
			}
		}
	}
}

// timeout is used to watch, unwatch and report timeouts.
type timeout struct {
	Key      interface{}    // what to watch, unwatch or what timed out
	Wait     time.Duration  // minimum duration to wait, or 0 for unwatch
	Deadline time.Time      // deadline, set at watch time
	Notify   chan<- timeout // channel for sending timeout notification
}
