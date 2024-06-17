// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
	_ "embed"
	"html/template"
	"sort"
)

// indexTemplate is the template for generating index.html files.
//
//go:embed index.html.tmpl
var indexTemplate string

// Index is a reporter that creates an index.html file for a Group.
//
// TODO implement Index reporter
type Index struct {
	To      string
	GroupBy string
	test    []*Test
}

// report implements multiReporter to gather the Tests.
func (x *Index) report(ctx context.Context, work resultRW, test *Test,
	data <-chan any) error {
	x.test = append(x.test, test)
	return nil
}

// stop implements multiStopper to generate the index file.
func (x *Index) stop(work resultRW) (err error) {
	t := template.New("Index")
	if t, err = t.Parse(indexTemplate); err != nil {
		return
	}
	w := work.Writer(x.To)
	defer func() {
		if e := w.Close(); e != nil && err == nil {
			err = e
		}
	}()
	err = t.Execute(w, x.templateData())
	return
}

// templateData returns the templateData for the index template.
func (x *Index) templateData() indexTemplateData {
	var d indexTemplateData
	for _, v := range x.groupValues() {
		if v != "" {
			g := indexGroup{Key: x.GroupBy, Value: v}
			for _, t := range x.test {
				if t.ID[x.GroupBy] == v {
					g.Test = append(g.Test, indexTest{t.ID, nil})
				}
			}
			d.Group = append(d.Group, g)
		} else {
			for _, t := range x.test {
				if t.ID[x.GroupBy] == "" {
					d.Test = append(d.Test, indexTest{t.ID, nil})
				}
			}
		}
	}
	return d
}

// groupValues returns the sorted, unique TestID values for the GroupBy key.
func (x *Index) groupValues() (val []string) {
	g := make(map[string]struct{})
	if x.GroupBy != "" {
		for _, t := range x.test {
			v := t.ID[x.GroupBy]
			g[v] = struct{}{}
		}
	} else {
		g[""] = struct{}{}
	}
	for k := range g {
		val = append(val, k)
	}
	sort.Strings(val)
	return
}

// indexTemplateData contains the data for indexTemplate execution.
type indexTemplateData struct {
	Group []indexGroup
	Test  []indexTest
}

// indexGroup contains the information for one group of Tests in the index.
type indexGroup struct {
	Key   string
	Value string
	Test  []indexTest
}

// indexTest contains the information for one Test in an indexGroup.
type indexTest struct {
	ID   TestID
	Link []indexLink
}

// indexLink contains the information for one link in an indexTest.
type indexLink struct {
	Name string
	Href string
}
