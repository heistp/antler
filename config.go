// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	_ "embed"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

//go:embed config.cue
var configCUE string

// Config is the Antler configuration, loaded from CUE.
type Config struct {
	Run TestRun // the top-level antler.Run instance
}

// LoadConfig uses the CUE API to load and return the Antler Config.
func LoadConfig(cuecfg *load.Config) (cfg *Config, err error) {
	inst := load.Instances([]string{}, cuecfg)[0]
	ctx := cuecontext.New()
	// compile config schema
	s := ctx.CompileString(configCUE, cue.Filename("config.cue"))
	if s.Err() != nil {
		err = s.Err()
		return
	}
	// compile data value from the CUE app instance
	d := ctx.BuildInstance(inst)
	if d.Err() != nil {
		err = d.Err()
		return
	}
	// unify data and schema into CUE value
	v := d.Unify(s)
	if v.Err() != nil {
		err = v.Err()
		return
	}
	cfg = &Config{}
	err = v.Decode(cfg)
	return
}
