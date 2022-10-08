// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	_ "embed"
	"html/template"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

//go:embed config.cue
var configCUE string

// Config is the Antler configuration, loaded from CUE.
type Config struct {
	// Run is the top-level TestRun instance.
	Run TestRun
}

// LoadConfig first executes templates in any .ant files to create the
// corresponding .cue files, then uses the CUE API to load and return the Antler
// Config.
func LoadConfig(cuecfg *load.Config) (cfg *Config, err error) {
	if err = executeConfigTemplates(); err != nil {
		return
	}
	// compile config schema
	ctx := cuecontext.New()
	s := ctx.CompileString(configCUE, cue.Filename("config.cue"))
	if s.Err() != nil {
		err = s.Err()
		return
	}
	// compile data value from the CUE app instance
	inst := load.Instances([]string{}, cuecfg)[0]
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

// executeConfigTemplates runs any .ant files as Go templates, to create their
// corresponding .cue files.
func executeConfigTemplates() (err error) {
	var ff []string
	if ff, err = filepath.Glob("*.ant"); err != nil {
		return
	}
	var t *template.Template
	for _, tf := range ff {
		if t, err = template.ParseFiles(tf); err != nil {
			return
		}
		var c *os.File
		if c, err = os.Create(tf[:len(tf)-4] + ".cue"); err != nil {
			return
		}
		defer c.Close()
		if err = t.Execute(c, nil); err != nil {
			return
		}
	}
	return
}
