// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"errors"
)

// tee confines a goroutine to receive data items from the returned in channel
// and send them to each out channel. tee returns when in is closed.
func tee(out ...chan any) (in chan any) {
	in = make(chan any, dataChanBufLen)
	go func() {
		for a := range in {
			for _, o := range out {
				o <- a
			}
		}
		for _, o := range out {
			close(o)
		}
	}()
	return
}

// mergeErr confines goroutines to combine errors from the given in channels to
// the out channel. The out channel is closed after all in channels are closed.
// mergeErr does not consume nil errors, so those are passed to out as well.
func mergeErr(in ...<-chan error) (out chan error) {
	out = make(chan error)
	ec := make(chan error)
	d := errors.New("done")
	for _, c := range in {
		go func(c <-chan error) {
			for e := range c {
				ec <- e
			}
			ec <- d
		}(c)
	}
	go func() {
		for n := len(in); n > 0; {
			e := <-ec
			if e == d {
				n--
				continue
			}
			out <- e
		}
		close(out)
	}()
	return
}
