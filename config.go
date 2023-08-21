// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/template"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/heistp/antler/node"
	"github.com/heistp/antler/node/metric"
	"gonum.org/v1/gonum/stat/distuv"
)

// templateExtension is the filename extension used for Go templates.
const templateExtension = ".tmpl"

//go:embed config.cue
var configCUE string

// Config is the Antler configuration, loaded from CUE.
type Config struct {
	// Run is the top-level TestRun instance.
	Run TestRun
}

// validate performs any programmatic validation on the Config that isn't
// possible to do with the schema in config.cue.
func (c *Config) validate() (err error) {
	if err = c.validateTestIDs(); err != nil {
		return
	}
	if err = c.validateNodeIDs(); err != nil {
		return
	}
	return
}

// validateTestIDs returns an error if any Test IDs are duplicated.
func (c *Config) validateTestIDs() (err error) {
	var ii, dd []TestID
	c.Run.VisitTests(func(t *Test) bool {
		f := func(i TestID) bool {
			return i.Equal(t.ID)
		}
		if slices.ContainsFunc(ii, f) {
			if !slices.ContainsFunc(dd, f) {
				dd = append(dd, t.ID)
			}
		} else {
			ii = append(ii, t.ID)
		}
		return true
	})
	if len(dd) > 0 {
		err = &DuplicateTestIDError{dd}
	}
	return
}

// DuplicateTestIDError is returned when multiple Tests have the same ID.
type DuplicateTestIDError struct {
	ID []TestID
}

func (d *DuplicateTestIDError) Error() string {
	var s []string
	for _, i := range d.ID {
		s = append(s, i.String())
	}
	return fmt.Sprintf("duplicate Test IDs: %s", strings.Join(s, ", "))
}

// validateNodeIDs returns an error if any Node IDs do not uniquely identify
// their fields.
func (c *Config) validateNodeIDs() (err error) {
	nn := make(map[node.Node]struct{})
	c.Run.VisitTests(func(t *Test) bool {
		r := node.NewTree(&t.Run)
		r.Walk(func(n node.Node) bool {
			nn[n] = struct{}{}
			return true
		})
		return true
	})
	ii := make(map[node.NodeID]struct{})
	var aa []node.NodeID
	for n := range nn {
		if _, ok := ii[n.ID]; ok {
			if !slices.Contains(aa, n.ID) {
				aa = append(aa, n.ID)
			}
		}
		ii[n.ID] = struct{}{}
	}
	if len(aa) > 0 {
		err = &AmbiguousNodeIDError{aa}
	}
	return
}

// AmbiguousNodeIDError is returned when multiple Nodes use the same ID but with
// different field values.
type AmbiguousNodeIDError struct {
	ID []node.NodeID
}

func (a *AmbiguousNodeIDError) Error() string {
	var s []string
	for _, i := range a.ID {
		s = append(s, i.String())
	}
	sort.Strings(s)
	return fmt.Sprintf("ambiguous Node IDs: %s", strings.Join(s, ", "))
}

// LoadConfig first executes templates in any .cue.tmpl files to create the
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
	if err = v.Decode(cfg); err != nil {
		return
	}
	err = cfg.validate()
	return
}

// executeConfigTemplates runs any .cue.tmpl files as Go templates, to create
// their corresponding .cue files.
func executeConfigTemplates() (err error) {
	var ff []string
	if ff, err = filepath.Glob("*.cue" + templateExtension); err != nil {
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
		if c, err = os.Create(tf[:len(tf)-len(templateExtension)]); err != nil {
			return
		}
		defer c.Close()
		if err = t.Execute(c, nil); err != nil {
			return
		}
	}
	return
}

// configFunc contains the template functions for .cue.tmpl config files.
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
func (f configFunc) lognRandFloat64(n int, p5, p95 float64) (
	sample []float64) {
	l5 := math.Log(p5)
	l95 := math.Log(p95)
	m := (l5 + l95) / 2
	s := (l95 - l5) / (2 * 1.645)
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
func (configFunc) jsonString(a any) (jsn string, err error) {
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
