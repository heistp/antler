// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package antler contains types for running the Antler application.

package antler

import (
	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node"
)

// Run runs Antler by loading the CUE Config and executing its top-level TestRun.
func Run(ctrl *node.Control) (err error) {
	var cfg *Config
	if cfg, err = LoadConfig(&load.Config{}); err != nil {
		return
	}
	s := reporterStack{}
	s.push([]reporter{&stdReporter{}})
	defer func() {
		if e := s.pop(); e != nil && err == nil {
			err = e
		}
	}()
	err = cfg.Run.do(ctrl, s)
	return
}
