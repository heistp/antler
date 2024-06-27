// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"html/template"
	"io"

	"github.com/heistp/antler/node"
)

// chartsTemplate is the template for Google Charts reporters.
//
//go:embed charts.html.tmpl
var chartsTemplate string

// chartsTemplateData contains the data for chartsTemplate execution.
type chartsTemplateData struct {
	Class   template.JS
	Data    chartsData
	Options map[string]any
	Stream  []StreamAnalysis
}

// ChartsTimeSeries is a reporter that makes time series plots using Google
// Charts.
type ChartsTimeSeries struct {
	// FlowLabel sets custom labels for Flows.
	FlowLabel map[node.Flow]string

	// To lists the names of files to execute the template to. A file of "-"
	// emits to stdout.
	To []string

	// Options is an arbitrary structure of Charts options, with defaults
	// defined in config.cue.
	// https://developers.google.com/chart/interactive/docs/gallery/linechart#configuration-options
	Options map[string]any
}

// report implements reporter
func (g *ChartsTimeSeries) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	t := template.New("Style")
	if t, err = t.Parse(styleTemplate); err != nil {
		return
	}
	t = t.New("ChartsTimeSeries")
	t = t.Funcs(template.FuncMap{
		"flowLabel": func(flow node.Flow) (label string) {
			label, ok := g.FlowLabel[flow]
			if !ok {
				return string(flow)
			}
			return label
		},
	})
	if t, err = t.Parse(chartsTemplate); err != nil {
		return
	}
	var a analysis
	for d := range in {
		out <- d
		switch v := d.(type) {
		case analysis:
			a = v
		}
	}
	td := chartsTemplateData{
		"google.visualization.LineChart",
		g.data(a.streams.byTime(), a.packets.byTime()),
		g.Options,
		a.streams.byTime(),
	}
	var ww []io.WriteCloser
	for _, to := range g.To {
		ww = append(ww, rw.Writer(to))
	}
	defer func() {
		for _, w := range ww {
			if e := w.Close(); e != nil && err == nil {
				err = e
			}
		}
	}()
	err = t.Execute(multiWriteCloser(ww...), td)
	return
}

// data returns the chart data.
func (g *ChartsTimeSeries) data(san []StreamAnalysis, pan []PacketAnalysis) (
	data chartsData) {
	var h chartsRow
	h.addColumn("")
	for _, d := range san {
		l := string(d.Client.Flow)
		if ll, ok := g.FlowLabel[d.Client.Flow]; ok {
			l = ll
		}
		h.addColumn(l)
	}
	for _, d := range pan {
		l := string(d.Client.Flow)
		if ll, ok := g.FlowLabel[d.Client.Flow]; ok {
			l = ll
		}
		h.addColumn(l)
	}
	data.addRow(h)
	for i, d := range san {
		for _, g := range d.GoodputPoint {
			var r chartsRow
			r.addColumn(g.T.Duration().Seconds())
			for j := 0; j < len(san); j++ {
				if j != i {
					r.addColumn(nil)
					continue
				}
				r.addColumn(g.Goodput.Mbps())
			}
			for j := 0; j < len(pan); j++ {
				r.addColumn(nil)
			}
			data.addRow(r)
		}
	}
	for i, d := range pan {
		for _, o := range d.OWD {
			var r chartsRow
			r.addColumn(o.T.Duration().Seconds())
			for j := 0; j < len(san); j++ {
				r.addColumn(nil)
			}
			for j := 0; j < len(pan); j++ {
				if j != i {
					r.addColumn(nil)
					continue
				}
				r.addColumn(float64(o.Delay) / 1000000)
			}
			data.addRow(r)
		}
	}
	return
}

// ChartsFCT is a reporter that makes time series plots using Google Charts.
type ChartsFCT struct {
	// To lists the names of files to execute the template to. A file of "-"
	// emits to stdout.
	To []string

	// Series matches Flows to series.
	Series []FlowSeries

	// Options is an arbitrary structure of Charts options, with defaults
	// defined in config.cue.
	// https://developers.google.com/chart/interactive/docs/gallery/scatterchart#configuration-options
	Options map[string]any
}

// report implements reporter
func (g *ChartsFCT) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	t := template.New("Style")
	if t, err = t.Parse(styleTemplate); err != nil {
		return
	}
	t = t.New("ChartsFCT")
	t = t.Funcs(template.FuncMap{})
	if t, err = t.Parse(chartsTemplate); err != nil {
		return
	}
	var a analysis
	for d := range in {
		out <- d
		switch v := d.(type) {
		case analysis:
			a = v
		}
	}
	if len(g.Series) == 0 {
		var f flows
		for _, s := range a.streams {
			f.add(s.Client.Flow)
		}
		g.Series = append(g.Series, FlowSeries{f.commonPrefix(), ".*", nil})
	}
	for i := 0; i < len(g.Series); i++ {
		s := &g.Series[i]
		if err = s.Compile(); err != nil {
			err = fmt.Errorf("regex error in series %s: %w", s.Name, err)
			return
		}
	}
	td := chartsTemplateData{
		"google.visualization.ScatterChart",
		g.data(a.streams.byTime()),
		g.Options,
		a.streams.byTime(),
	}
	var ww []io.WriteCloser
	for _, to := range g.To {
		ww = append(ww, rw.Writer(to))
	}
	defer func() {
		for _, w := range ww {
			if e := w.Close(); e != nil && err == nil {
				err = e
			}
		}
	}()
	err = t.Execute(multiWriteCloser(ww...), td)
	return
}

// data returns the chart data.
func (g *ChartsFCT) data(san []StreamAnalysis) (data chartsData) {
	var h chartsRow
	h.addColumn("")
	for _, s := range g.Series {
		h.addColumn(s.Name)
	}
	data.addRow(h)
	for _, a := range san {
		var r chartsRow
		r.addColumn(a.Length.Kilobytes())
		for _, s := range g.Series {
			if s.Match(a.Client.Flow) {
				r.addColumn(a.FCT.Seconds())
			} else {
				r.addColumn(nil)
			}
		}
		data.addRow(r)
	}
	return
}

// FlowSeries groups flows into series by matching the Flow ID with a Regex.
type FlowSeries struct {
	Name    string
	Pattern string
	rgx     *regexp.Regexp
}

// Compile compiles Pattern to a Regexp.
func (s *FlowSeries) Compile() (err error) {
	s.rgx, err = regexp.Compile(s.Pattern)
	return
}

// Match returns true if Flow matches Regex.
func (s *FlowSeries) Match(flow node.Flow) (matches bool) {
	return s.rgx.MatchString(string(flow))
}

// chartsData represents tabular data for use in Google Charts.
type chartsData [][]any

// addRow adds a row to the data.
func (c *chartsData) addRow(row chartsRow) {
	*c = append(*c, row)
}

// chartsRow represents the data for a single row.
type chartsRow []any

// addColumn adds a column to the row.
func (r *chartsRow) addColumn(v any) {
	*r = append(*r, v)
}

// flows wraps []node.Flow with additional functionality.
type flows []node.Flow

// add adds a Flow.
func (f *flows) add(flow node.Flow) {
	(*f) = append(*f, flow)
}

// sort sorts the Flows lexically.
func (f *flows) sort() {
	sort.Slice(*f, func(i, j int) bool {
		return string((*f)[i]) < string((*f)[j])
	})
}

// strings returns the Flows as strings.
func (f *flows) strings() (s []string) {
	s = make([]string, 0, len(*f))
	for _, n := range *f {
		s = append(s, string(n))
	}
	return
}

// commonPrefix returns the longest common prefix to all flows.
func (f *flows) commonPrefix() (prefix string) {
	if len(*f) == 0 {
		return
	}
	s := f.strings()
	sort.Strings(s)
	r := s[0]
	l := s[len(s)-1]
	for i := 0; i < len(r); i++ {
		if l[i] == r[i] {
			prefix += string(l[i])
		} else {
			break
		}
	}
	prefix = strings.TrimRightFunc(prefix, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return
}
