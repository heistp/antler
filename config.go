// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node/metric"
	"gonum.org/v1/gonum/stat/distuv"
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
	f := configFunc{}
	var t *template.Template
	for _, tf := range ff {
		t = template.New(tf).Funcs(f.funcMap())
		if t, err = t.ParseFiles(tf); err != nil {
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

// configFunc contains the template functions for .ant config files.
type configFunc struct {
}

// expRandFloat64 returns a list of n random numbers on an exponential
// distribution, with the given rate parameter (1.0 is a useful default).
func (f configFunc) expRandFloat64(n int, rate float64) (sample []float64) {
	d := distuv.Exponential{Rate: rate}
	for i := 0; i < n; i++ {
		sample = append(sample, d.Rand())
	}
	return
}

// expRand returns a list of n random numbers on an exponential distribution,
// with the given rate parameter (1.0 is a useful default).
func (f configFunc) expRand(n int, rate float64) (jsn string, err error) {
	jsn, err = f.jsonString(f.expRandFloat64(n, rate))
	return
}

// expRandDuration returns a list of n random durations, deviating from a mean
// duration on an exponential distribution, with the given rate parameter (1.0
// is a useful default).
func (f configFunc) expRandDuration(meanDuration string, n int, rate float64) (
	jsn string, err error) {
	var m time.Duration
	if m, err = time.ParseDuration(meanDuration); err != nil {
		return
	}
	r := f.expRandFloat64(n, rate)
	var s []string
	for _, v := range r {
		d := time.Duration(v * float64(m))
		s = append(s, d.String())
	}
	jsn, err = f.jsonString(s)
	return
}

// lognRandFloat64 returns a list of n random numbers on a lognormal
// distribution, with the given 5th and 95th percentile values.
func (f configFunc) lognRandFloat64(n int, log5, log95 float64) (
	sample []float64) {
	m := (log5 + log95) / 2
	s := (log95 - log5) / (2 * 1.645)
	d := distuv.LogNormal{Mu: m, Sigma: s}
	for i := 0; i < n; i++ {
		sample = append(sample, d.Rand())
	}
	return
}

// lognRand returns a list of n random number on a lognormal distribution, with
// the given 5th and 95th percentile values.
func (f configFunc) lognRand(n int, p5, p95 float64) (
	jsn string, err error) {
	jsn, err = f.jsonString(f.lognRandFloat64(n, p5, p95))
	return
}

// lognRandBytes returns a list of n random bytes on a lognormal distribution,
// with the given 5th and 95th percentile values.
func (f configFunc) lognRandBytes(n int, p5, p95 metric.Bytes) (
	jsn string, err error) {
	r := f.lognRandFloat64(n, float64(p5), float64(p95))
	var b []metric.Bytes
	for _, v := range r {
		b = append(b, metric.Bytes(v))
	}
	jsn, err = f.jsonString(b)
	return
}

// jsonString marshals 'a' as JSON into a string.
func (configFunc) jsonString(a interface{}) (jsn string, err error) {
	var b []byte
	if b, err = json.Marshal(a); err != nil {
		return
	}
	jsn = string(b)
	return
}

// funcMap returns the function map with all configFunc functions.
func (f configFunc) funcMap() template.FuncMap {
	return template.FuncMap{
		"expRand":         f.expRand,
		"expRandDuration": f.expRandDuration,
		"lognRand":        f.lognRand,
		"lognRandBytes":   f.lognRandBytes,
	}
}
