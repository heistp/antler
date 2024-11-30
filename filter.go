// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

package antler

import (
	"fmt"
	"regexp"
	"strings"
)

// A TestFilter accepts or rejects Tests.
type TestFilter interface {
	Accept(*Test) bool
}

// AndFilter accepts a Test if each of its TestFilters accepts it. AndFilter
// panics if it has no TestFilters.
type AndFilter []TestFilter

// Accept implements TestFilter.
func (a AndFilter) Accept(test *Test) bool {
	if len(a) == 0 {
		panic("AndFilter requires at least one TestFilter")
	}
	for _, f := range a {
		if !f.Accept(test) {
			return false
		}
	}
	return true
}

// OrFilter accepts a Test if any of its TestFilters accepts it. OrFilter panics
// if it has no TestFilters.
type OrFilter []TestFilter

// Accept implements TestFilter
func (o OrFilter) Accept(test *Test) bool {
	if len(o) == 0 {
		panic("OrFilter requires at least one TestFilter")
	}
	for _, f := range o {
		if f.Accept(test) {
			return true
		}
	}
	return false
}

// RegexFilter is a TestFilter that matches Tests by their ID using regular
// expressions. If any of a Test ID's key/value pairs match the non-nil
// expressions in Key and Value, the Test is accepted. If both Key and Value are
// nil (i.e. a zero value RegexFilter), all Tests are accepted.
type RegexFilter struct {
	Key   *regexp.Regexp
	Value *regexp.Regexp
}

// NewRegexFilterArg returns a new RegexFilter from a string argument. The
// argument may be either a single pattern matching the value of any ID field,
// or a string in the form key=value, where key and value are separate patterns
// that must match both a Test ID key and value for it to be accepted.
func NewRegexFilterArg(arg string) (flt *RegexFilter, err error) {
	flt = &RegexFilter{}
	s := strings.Split(arg, "=")
	switch len(s) {
	case 1:
		flt.Value, err = regexp.Compile(s[0])
	case 2:
		if flt.Key, err = regexp.Compile(s[0]); err != nil {
			return
		}
		flt.Value, err = regexp.Compile(s[1])
	default:
		err = fmt.Errorf("invalid key=value regex filter arg: '%s'", arg)
	}
	return
}

// Accept implements TestFilter
func (f *RegexFilter) Accept(test *Test) bool {
	for k, v := range test.ID {
		if (f.Key == nil || f.Key.MatchString(k)) &&
			(f.Value == nil || f.Value.MatchString(v)) {
			return true
		}
	}
	return false
}

// BoolFilter is a TestFilter that accepts (if true) or rejects all Tests.
type BoolFilter bool

// Accept implements TestFilter.
func (b BoolFilter) Accept(test *Test) bool {
	return bool(b)
}
