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

// Scenario is used to form a hierarchy of Tests. Each Scenario is a node in the
// hierarchy containing a list of Tests with the same ID keys, and a list of
// sub-Scenarios.
type Scenario struct {
	// Name is the name of the Scenario, and is used as the name of the
	// directory containing the results for the Scenario.
	Name string

	// Path is the output path for the Scenario, relative to the results
	// directory.  This is assigned at Config load time.
	Path string

	// ResultPrefix is the base file name for any output files. It may use Go
	// template syntax, and is further documented in config.cue.
	// TODO Further document ResultPrefix this after the template is executed
	// and the results stored in Test.
	ResultPrefix string

	// IDInfo maps Test ID keys to information about the key/value pair.
	IDInfo map[string]IDInfo

	// Test lists the Tests in the Scenario, and may be empty for Scenarios that
	// only contain other Scenarios.
	Test []Test

	// Scenario lists any sub-Scenarios.
	Scenario []Scenario

	// During is a pipeline of Reports run while the Tests run.
	During Report

	// DuringDefault is the default pipeline of Reports run while the Tests run.
	DuringDefault Report

	// After is a pipeline of Reports run after the Tests complete.
	After Report

	// After is the default pipeline of Reports run after the Tests complete.
	AfterDefault Report
}

// IDInfo contains information about one key/value pair in a Test ID.
type IDInfo struct {
	Key   string
	Title string
}

// VisitTests calls the given visitor func for each Test in the Scenario
// hierarchy. The visitor may return false to stop visiting, in which case
// VisitTests will also return false.
func (s *Scenario) VisitTests(visitor func(*Test) bool) bool {
	for _, t := range s.Test {
		if !visitor(&t) {
			return false
		}
	}
	for _, s := range s.Scenario {
		if !s.VisitTests(visitor) {
			return false
		}
	}
	return true
}

// do runs a doer on the Tests, and recursively on the sub-Scenarios.
func (s *Scenario) do(ctx context.Context, d doer2) (err error) {
	for _, t := range s.Test {
		if err = d.do(ctx, &t); err != nil {
			return
		}
	}
	for _, s := range s.Scenario {
		if err = s.do(ctx, d); err != nil {
			return
		}
	}
	return
}

// setPath is called recursively to set the Path fields from the Names.
func (s *Scenario) setPath(prefix string) {
	s.Path = filepath.Join(prefix, s.Name)
	for _, c := range s.Scenario {
		c.setPath(s.Path)
	}
}

// generateResultPrefixes is called recursively to execute the ResultPrefix
// template for each Test, to set their ResultPrefixX fields.
// TODO update generateResultPrefixes doc after ResultPrefixX -> ResultPrefix
func (s *Scenario) generateResultPrefixes() (err error) {
	m := template.New("ResultPrefix")
	if m, err = m.Parse(s.ResultPrefix); err != nil {
		return
	}
	pp := make(map[string]int)
	var d []string
	for _, t := range s.Test {
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
	for _, c := range s.Scenario {
		if err = c.generateResultPrefixes(); err != nil {
			return
		}
	}
	return
}

// DuplicateResultPrefixError2 is returned when multiple Tests have the same
// ResultPrefix.
// TODO rename DuplicateResultPrefixError2 after Scenarios are in place
type DuplicateResultPrefixError2 struct {
	Path   string
	Prefix []string
}

// Error implements error
func (d DuplicateResultPrefixError2) Error() string {
	return fmt.Sprintf("scenario %s contains duplicate Test ResultPrefixes: %s",
		d.Path, strings.Join(d.Prefix, ", "))
}

// validateTestIDs is called recursively to check that Test IDs contain the
// fields listed in IDInfo, and that no Test IDs are duplicated in a Scenario.
func (s *Scenario) validateTestIDs() (err error) {
	var ii, dd []TestID
	for _, t := range s.Test {
		for k := range t.ID {
			if _, ok := s.IDInfo[k]; !ok {
				err = TestIDError{s.Path, t.ID}
				return
			}
		}
		for k := range s.IDInfo {
			if _, ok := t.ID[k]; !ok {
				err = TestIDError{s.Path, t.ID}
				return
			}
		}
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
	for _, c := range s.Scenario {
		if err = c.validateTestIDs(); err != nil {
			return
		}
	}
	return
}

// TestIDError is returned when a Test's ID does not match the Scenario's
// IDInfo.
type TestIDError struct {
	Path string
	ID   TestID
}

// Error implements error
func (e TestIDError) Error() string {
	return fmt.Sprintf("scenario %s contains Test with ID %s that does not "+
		"match the scenario's IDInfo field", e.Path, e.ID)
}

// DuplicateTestIDError2 is returned when multiple Tests have the same ID.
// TODO rename DuplicateTestIDError2 after Scenarios are in place
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
	return fmt.Sprintf("scenario %s contains duplicate Test IDs: %s",
		d.Path, strings.Join(s, ", "))
}

// A doer2 takes action on a Test, visited in a Scenario hierarchy.
type doer2 interface {
	do(context.Context, *Test) error
}
