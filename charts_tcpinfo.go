// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"html/template"
	"io"
	"os"

	"github.com/heistp/antler/node"
)

// ChartsTCPInfo is a reporter that makes TCPInfo plots using Google Charts.
type ChartsTCPInfo struct {
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
func (n *ChartsTCPInfo) report(in reportIn) {
	var f simpleReportFunc = n.reportOne
	f.report(in)
}

// report runs one time series report.
func (n *ChartsTCPInfo) reportOne(in reportIn) (err error) {
	var w io.WriteCloser
	defer func() {
		if w != nil && w != os.Stdout {
			w.Close()
		}
	}()
	t := template.New("ChartsTCPInfo")
	t = t.Funcs(template.FuncMap{
		"flowLabel": func(flow node.Flow) (label string) {
			label, ok := n.FlowLabel[flow]
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
	for d := range in.data {
		switch v := d.(type) {
		case analysis:
			a = v
		}
	}
	td := chartsTemplateData{
		"google.visualization.LineChart",
		n.data(a.streams.byTime()),
		n.Options,
	}
	var ww []io.Writer
	for _, to := range n.To {
		if to == "-" {
			w = os.Stdout
		} else if w, err = os.Create(in.test.outPath(to)); err != nil {
			return
		}
		ww = append(ww, w)
	}
	err = t.Execute(io.MultiWriter(ww...), td)
	return
}

// data returns the chart data.
func (n *ChartsTCPInfo) data(san []streamAnalysis) (data chartsData) {
	var h chartsRow
	h.addColumn("")
	for _, d := range san {
		l := string(d.Client.Flow)
		if ll, ok := n.FlowLabel[d.Client.Flow]; ok {
			l = ll
		}
		h.addColumn(l)
		h.addColumn(l)
	}
	data.addRow(h)
	for i, d := range san {
		for _, n := range d.TCPInfo {
			var r chartsRow
			r.addColumn(n.T.Duration().Seconds())
			for j := 0; j < len(san); j++ {
				if j != i {
					r.addColumn(nil)
					r.addColumn(nil)
					continue
				}
				r.addColumn(n.TotalRetransmits)
				r.addColumn(n.SendCwnd)
			}
			data.addRow(r)
		}
	}
	return
}
