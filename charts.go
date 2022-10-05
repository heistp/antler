// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	_ "embed"

	"html/template"
	"io"
	"os"

	"github.com/heistp/antler/node"
)

// chartsTimeSeriesTemplate is the template for the ChartsTimeSeries reporter.
//
//go:embed charts_time_series.tmpl
var chartsTimeSeriesTemplate string

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
	Options map[string]interface{}
}

// report implements reporter
func (g *ChartsTimeSeries) report(in reportIn) {
	var f simpleReportFunc = g.reportOne
	f.report(in)
}

// report runs one time series report.
func (g *ChartsTimeSeries) reportOne(in reportIn) (err error) {
	type tdata struct {
		ChartsTimeSeries
		Data    chartsData
		Options map[string]interface{}
	}
	var w io.WriteCloser
	defer func() {
		if w != nil && w != os.Stdout {
			w.Close()
		}
	}()
	t := template.New("ChartsTimeSeries")
	t = t.Funcs(template.FuncMap{
		"flowLabel": func(flow node.Flow) (label string) {
			label, ok := g.FlowLabel[flow]
			if !ok {
				return string(flow)
			}
			return label
		},
	})
	if t, err = t.Parse(chartsTimeSeriesTemplate); err != nil {
		return
	}
	d := newReportData()
	for a := range in.data {
		d.add(a)
	}
	d.analyze()
	td := tdata{*g, g.data(d.streams.byTime(), d.packets.byTime()), g.Options}
	var ww []io.Writer
	for _, to := range g.To {
		if to == "-" {
			w = os.Stdout
		} else if w, err = os.Create(to); err != nil {
			return
		}
		ww = append(ww, w)
	}
	err = t.Execute(io.MultiWriter(ww...), td)
	return
}

// data returns the chart data.
func (g *ChartsTimeSeries) data(sdata []streamData, pdata []packetData) (
	data chartsData) {
	var h chartsRow
	h.addColumn("")
	for _, d := range sdata {
		l := string(d.Client.Flow)
		if ll, ok := g.FlowLabel[d.Client.Flow]; ok {
			l = ll
		}
		h.addColumn(l)
	}
	for _, d := range pdata {
		l := string(d.Client.Flow)
		if ll, ok := g.FlowLabel[d.Client.Flow]; ok {
			l = ll
		}
		h.addColumn(l)
	}
	data.addRow(h)
	for i, d := range sdata {
		for _, g := range d.Goodput {
			var r chartsRow
			r.addColumn(g.T.Duration().Seconds())
			for j := 0; j < len(sdata); j++ {
				if j != i {
					r.addColumn(nil)
					continue
				}
				r.addColumn(g.Goodput.Mbps())
			}
			for j := 0; j < len(pdata); j++ {
				r.addColumn(nil)
			}
			data.addRow(r)
		}
	}
	for i, d := range pdata {
		for _, o := range d.OWD {
			var r chartsRow
			r.addColumn(o.T.Duration().Seconds())
			for j := 0; j < len(sdata); j++ {
				r.addColumn(nil)
			}
			for j := 0; j < len(pdata); j++ {
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

// chartsData represents tabular data for use in Google Charts.
type chartsData [][]interface{}

// addRow adds a row to the data.
func (c *chartsData) addRow(row chartsRow) {
	*c = append(*c, row)
}

// chartsRow represents the data for a single row.
type chartsRow []interface{}

// addColumn adds a column to the row.
func (r *chartsRow) addColumn(v interface{}) {
	*r = append(*r, v)
}
