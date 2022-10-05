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
	// Title is the plot title.
	Title string

	// VTitle is the title of the vertical axis.
	VTitle string

	// VMin is the minimum value on the vertical axis.
	VMin int

	// VMax is the maximum value on the vertical axis.
	VMax int

	// FlowLabel sets custom labels for Flows.
	FlowLabel map[node.Flow]string

	// To is the name of a file to execute the template to.
	To string

	// Stdout, if true, also writes the template to stdout.
	Stdout bool
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
		Stream []streamData
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
	s := newStreams()
	for a := range in.data {
		switch v := a.(type) {
		case node.StreamInfo:
			d := s.data(v.Flow)
			d.Info = v
		case node.StreamIO:
			d := s.data(v.Flow)
			if v.Sent {
				d.Sent = append(d.Sent, v)
			} else {
				d.Rcvd = append(d.Rcvd, v)
			}
		}
	}
	s.analyze()
	d := tdata{*g, s.byTime()}
	var ww []io.Writer
	if w, err = os.Create(g.To); err != nil {
		return
	}
	ww = append(ww, w)
	if g.Stdout {
		ww = append(ww, os.Stdout)
	}
	err = t.Execute(io.MultiWriter(ww...), d)
	return
}
