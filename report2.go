// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

package antler

import "context"

// A reporter2 can process data items from the node for multiple Tests in a
// Scenario. It is run as a stage in a pipeline, where one reportData is passed
// to the data channel for each Test.
//
// Reporters should be able to handle multiple Test data streams concurrently.
//
// Reporters may use the given Context to react to cancellation signals, and if
// canceled, should return the error from context.Cause(ctx). Reporters may also
// ignore the Context. In any case, they should expect that partial input data
// is possible, in which case an error should be returned.
//
// If a reporter runs in the first stage of a pipeline with no input, each in
// channel will be closed immediately with no input data.
//
// If a reporter runs in the last stage of a pipeline, it may send nothing to
// out. However, for configuration flexibility, most reports should forward to
// out, unless they are certain to be the last stage in the pipeline.
//
// Reporters may return with or without an error, however, they must not do so
// until they are completely done their work, and any goroutines started have
// completed. Any remaining data on the reportData in channels will be forwarded
// to their out channels.
type reporter2 interface {
	report(ctx context.Context, data <-chan reportData) error
}

// reportData contains the information reporter implementations need to read
// data and write reports for one Test.
type reportData struct {
	test *Test
	in   <-chan any
	out  chan<- any
	rw   rwer
}
