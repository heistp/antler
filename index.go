// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
	_ "embed"
	"html/template"
	"path/filepath"
	"sort"
	"sync"
)

// indexTemplate is the template for generating index.html files.
//
//go:embed index.html.tmpl
var indexTemplate string

// Index is a reporter that creates an index.html file for a Group.
type Index struct {
	To      string
	GroupBy string
	Title   string
	test    []*Test
	sync.Mutex
}

// report implements multiReporter to gather the Tests.
func (x *Index) report(ctx context.Context, work resultRW, test *Test,
	data <-chan any) error {
	x.Lock()
	x.test = append(x.test, test)
	x.Unlock()
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
	err = t.Execute(w, x.templateData(work.stat.Paths()))
	return
}

// templateData returns the templateData for the index template.
func (x *Index) templateData(path pathSet) indexTemplateData {
	var d indexTemplateData
	d.Title = x.Title
	for _, v := range x.groupValues() {
		g := indexGroup{Key: x.GroupBy, Value: v}
		c := make(map[string]struct{})
		for _, t := range x.test {
			if t.ID[x.GroupBy] != v {
				continue
			}
			var l []indexLink
			for _, p := range path.withPrefix(t.Path).sorted() {
				l = append(l, indexLink{filepath.Base(p), p})
			}
			g.Test = append(g.Test, indexTest{t.ID, l})
			for k := range t.ID {
				c[k] = struct{}{}
			}
		}
		delete(c, x.GroupBy)
		for k := range c {
			g.Column = append(g.Column, k)
		}
		sort.Strings(g.Column)
		g.Column = append([]string{x.GroupBy}, g.Column...)
		d.Group = append(d.Group, g)
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
	Title string
	Group []indexGroup
}

// indexGroup contains the information for one group of Tests in the index.
type indexGroup struct {
	Key    string
	Value  string
	Column []string
	Test   []indexTest
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
