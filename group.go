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

	// Test lists the Tests in the Group, and may be empty for Groups that only
	// contain other Groups.
	Test []Test

	// Sub lists any sub-Groups.
	Sub []Group
}

// Visit calls the given visitor for this Group, its Tests, and any sub-Groups
// and their Tests. The given visitor may implement Grouper, Tester or both to
// be called for the corresponding type. The Group method will be called before
// the Test method for any Tests in a Group. If any method returns an error,
// visiting stops and that error is returned. The given Context is passed to
// the Group and Test methods.
func (s *Group) Visit(ctx context.Context, visitor any) (err error) {
	if g, ok := visitor.(Grouper); ok {
		if err = g.Group(ctx, s); err != nil {
			return
		}
	}
	if t, ok := visitor.(Tester); ok {
		for i := range s.Test {
			if err = t.Test(ctx, &s.Test[i]); err != nil {
				return
			}
		}
	}
	for i := range s.Sub {
		if err = s.Sub[i].Visit(ctx, visitor); err != nil {
			return
		}
	}
	return
}

// VisitFunc calls the given functions for this Group and any sub-Groups,
// recursively, and all of their Tests. The functions are called according to
// the Visit method. Either function may be nil, in which case it isn't called.
// The Context in the Visitor interface is not used, and will be passed as nil.
func (s *Group) VisitFunc(group func(*Group) error,
	test func(*Test) error) error {
	return s.Visit(nil, visitor{group, test})
}

// visitor is an internal Visitor used for VisitFunc.
type visitor struct {
	group func(*Group) error
	test  func(*Test) error
}

// Group implements Visitor.
func (v visitor) Group(ctx context.Context, group *Group) error {
	if v.group == nil {
		return nil
	}
	return v.group(group)
}

// Test implements Visitor.
func (v visitor) Test(ctx context.Context, test *Test) error {
	if v.test == nil {
		return nil
	}
	return v.test(test)
}

// A Visitor is called to do something with Groups and Tests. The Group method
// is called before the Test method for Tests in that Group. The Context is
// the one passed to the Group.Visit method, and may be nil. If an error is
// returned by either of the methods, the visiting stops and the Group.Visit
// method returns that error.
type Visitor interface {
	Group(context.Context, *Group) error
	Test(context.Context, *Test) error
}

// A Grouper may be called by the Visit method to do something with a Group.
type Grouper interface {
	Group(context.Context, *Group) error
}

// A Tester may be called by the Visit method to do something with a Test.
type Tester interface {
	Test(context.Context, *Test) error
}

// setPath is called recursively to set the Path fields from the Names.
func (s *Group) setPath(prefix string) {
	s.Path = filepath.Join(prefix, s.Name)
	for i := range s.Sub {
		s.Sub[i].setPath(s.Path)
	}
}

// setTestGroup is called recursively to set the Group field for all Tests.
func (s *Group) setTestGroup() {
	for i := range s.Test {
		s.Test[i].Group = s
	}
	for i := range s.Sub {
		s.Sub[i].setTestGroup()
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
	for i := range s.Test {
		t := &s.Test[i]
		var b strings.Builder
		if err = m.Execute(&b, t.ID); err != nil {
			return
		}
		p := b.String()
		if p != "" {
			t.ResultPrefixX = filepath.Join(s.Path, p)
		} else {
			t.ResultPrefixX = s.Path + string(filepath.Separator)
		}
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
	for _, c := range s.Sub {
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
	for _, c := range s.Sub {
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
