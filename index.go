// SPDX-License-Identifier: GPL-3.0-or-later
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
	To          string
	GroupBy     string
	Title       string
	ExcludeFile []string
	test        []*Test
	sync.Mutex
}

// report implements multiReporter to gather the Tests.
func (i *Index) report(ctx context.Context, work resultRW, test *Test,
	data <-chan any) error {
	i.Lock()
	i.test = append(i.test, test)
	i.Unlock()
	return nil
}

// stop implements multiStopper to generate the index file.
func (i *Index) stop(work resultRW) (err error) {
	t := template.New("Style")
	if t, err = t.Parse(styleTemplate); err != nil {
		return
	}
	t = t.New("Index")
	if t, err = t.Parse(indexTemplate); err != nil {
		return
	}
	w := work.Writer(i.To)
	defer func() {
		if e := w.Close(); e != nil && err == nil {
			err = e
		}
	}()
	var d indexTemplateData
	if d, err = i.templateData(work.Paths()); err != nil {
		return
	}
	err = t.Execute(w, d)
	return
}

// templateData returns the templateData for the index template.
func (i *Index) templateData(paths pathSet) (data indexTemplateData, err error) {
	data.Title = i.Title
	data.GroupBy = i.GroupBy
	for _, v := range i.groupValues() {
		g := indexGroup{Key: i.GroupBy, Value: v}
		c := make(map[string]struct{})
		for _, t := range i.test {
			if t.ID[i.GroupBy] != v {
				continue
			}
			var l []indexLink
			for _, p := range paths.withPrefix(t.Path).sorted() {
				var x bool
				if x, err = i.excludeFile(p); err != nil {
					return
				}
				if !x {
					l = append(l, indexLink{filepath.Base(p), p})
				}
			}
			g.Test = append(g.Test, indexTest{t.ID, l})
			for k := range t.ID {
				c[k] = struct{}{}
			}
		}
		delete(c, i.GroupBy)
		for k := range c {
			g.Column = append(g.Column, k)
		}
		sort.Strings(g.Column)
		if i.GroupBy != "" {
			g.Column = append([]string{i.GroupBy}, g.Column...)
		}
		data.Group = append(data.Group, g)
	}
	return
}

// excludeFile returns true if the base name of the given path matches any of
// the ExcludeFile patterns.
func (i *Index) excludeFile(path string) (matched bool, err error) {
	b := filepath.Base(path)
	for _, p := range i.ExcludeFile {
		if matched, err = filepath.Match(p, b); err != nil || matched {
			return
		}
	}
	return
}

// groupValues returns the sorted, unique TestID values for the GroupBy key.
func (i *Index) groupValues() (val []string) {
	g := make(map[string]struct{})
	if i.GroupBy != "" {
		for _, t := range i.test {
			v := t.ID[i.GroupBy]
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
	Title   string
	Group   []indexGroup
	GroupBy string
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
