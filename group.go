// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import (
	"context"
	"fmt"
	"html/template"
	"path/filepath"
	"slices"
	"strings"
)

// Group is used to form a hierarchy of Tests. Each Group is a node in the
// hierarchy containing a list of Tests with the same ID keys, and a list of
// sub-Groups.
type Group struct {
	// Name is the name of the Group, and is used as the name of the directory
	// containing the results for the Group.
	Name string

	// Path is the output path for the Group, relative to the results directory.
	// This is assigned at Config load time.
	Path string

	// ResultPrefix is the base file name for any output files. It may use Go
	// template syntax, and is further documented in config.cue.
	// TODO Further document ResultPrefix this after the template is executed
	// and the results stored in Test.
	ResultPrefix string

	// IDInfo maps Test ID keys to information about the key/value pair.
	IDInfo map[string]IDInfo

	// Test lists the Tests in the Group, and may be empty for Groups that only
	// contain other Groups.
	Test []Test

	// Group lists any sub-Groups.
	Group []Group

	// During is a pipeline of Reports run while the Tests run.
	During Report

	// After is a pipeline of Reports run after the Tests complete.
	After Report
}

// IDInfo contains information about one key/value pair in a Test ID.
type IDInfo struct {
	Key   string
	Title string
}

// VisitTests calls the given visitor func for each Test in the Group hierarchy.
// The visitor may return false to stop visiting, in which case VisitTests will
// also return false.
func (s *Group) VisitTests(visitor func(*Test) bool) bool {
	for _, t := range s.Test {
		if !visitor(&t) {
			return false
		}
	}
	for _, s := range s.Group {
		if !s.VisitTests(visitor) {
			return false
		}
	}
	return true
}

// do runs a doer on the Tests, and recursively on the sub-Groups.
func (s *Group) do(ctx context.Context, d doer2) (err error) {
	for _, t := range s.Test {
		if err = d.do(ctx, &t); err != nil {
			return
		}
	}
	for _, s := range s.Group {
		if err = s.do(ctx, d); err != nil {
			return
		}
	}
	return
}

// setPath is called recursively to set the Path fields from the Names.
func (s *Group) setPath(prefix string) {
	s.Path = filepath.Join(prefix, s.Name)
	for i := range s.Group {
		s.Group[i].setPath(s.Path)
	}
}

// setTestGroup is called recursively to set the Group field for all Tests.
func (s *Group) setTestGroup() {
	for i := range s.Test {
		s.Test[i].Group = s
	}
	for _, s := range s.Group {
		s.setTestGroup()
	}
}

// generateResultPrefixes is called recursively to execute the ResultPrefix
// template for each Test, to set their ResultPrefixX fields.
// TODO update generateResultPrefixes doc after ResultPrefixX -> ResultPrefix
func (s *Group) generateResultPrefixes() (err error) {
	m := template.New("ResultPrefix")
	if m, err = m.Parse(s.ResultPrefix); err != nil {
		return
	}
	pp := make(map[string]int)
	var d []string
	for i := 0; i < len(s.Test); i++ {
		t := &s.Test[i]
		var b strings.Builder
		if err = m.Execute(&b, t.ID); err != nil {
			return
		}
		p := b.String()
		t.ResultPrefixX = p
		if v, ok := pp[p]; ok {
			if v == 1 {
				d = append(d, p)
			}
			pp[p] = v + 1
		} else {
			pp[p] = 1
		}
	}
	if len(d) > 0 {
		err = DuplicateResultPrefixError2{s.Path, d}
		return
	}
	for _, c := range s.Group {
		if err = c.generateResultPrefixes(); err != nil {
			return
		}
	}
	return
}

// DuplicateResultPrefixError2 is returned when multiple Tests have the same
// ResultPrefix.
// TODO rename DuplicateResultPrefixError2 after Groups are in place
type DuplicateResultPrefixError2 struct {
	Path   string
	Prefix []string
}

// Error implements error
func (d DuplicateResultPrefixError2) Error() string {
	return fmt.Sprintf("Group %s contains duplicate Test ResultPrefixes: %s",
		d.Path, strings.Join(d.Prefix, ", "))
}

// validateTestIDs is called recursively to check that no Test IDs are
// duplicated in a Group.
func (s *Group) validateTestIDs() (err error) {
	var ii, dd []TestID
	for _, t := range s.Test {
		f := func(id TestID) bool {
			return id.Equal(t.ID)
		}
		if slices.ContainsFunc(ii, f) {
			if !slices.ContainsFunc(dd, f) {
				dd = append(dd, t.ID)
			}
		} else {
			ii = append(ii, t.ID)
		}
	}
	if len(dd) > 0 {
		err = DuplicateTestIDError2{s.Path, dd}
		return
	}
	for _, c := range s.Group {
		if err = c.validateTestIDs(); err != nil {
			return
		}
	}
	return
}

// DuplicateTestIDError2 is returned when multiple Tests have the same ID.
// TODO rename DuplicateTestIDError2 after Groups are in place
type DuplicateTestIDError2 struct {
	Path string
	ID   []TestID
}

// Error implements error
func (d DuplicateTestIDError2) Error() string {
	var s []string
	for _, i := range d.ID {
		s = append(s, i.String())
	}
	return fmt.Sprintf("Group %s contains duplicate Test IDs: %s",
		d.Path, strings.Join(s, ", "))
}

// A doer2 takes action on a Test, visited in a Group hierarchy.
type doer2 interface {
	do(context.Context, *Test) error
}
