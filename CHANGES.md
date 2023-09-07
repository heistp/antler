# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to
[Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Implement pipelined reports (`TestRun.Report`, `Test.During`, `Test.Report`)
- Turn Analyze into a report and add it to examples that need it
- Implement log sorting
- Write test results non-destructively
- Validate ResultPrefixes are unique
- Add embedded web server to serve results (`server` command)

### Changed

- Replace node.Control with context.WithCancelCause from Go 1.20
- Change usages of interface{} to the `any` alias from Go 1.18
- Remove conn.Close and simplify connection closure
- Rename node.NodeID to node.ID to reduce stutter
- Rename Test.OutputPrefix to Test.ResultPrefix

### Fixed

- Fix hang after Go runtime panic in node
- Propagate parent context to node.runs goroutine
- Fix panic in FCT analysis when no data points are available
- Fix one second cancellation delay for Stream tests (check Context in receive)
- Consistently cancel Contexts in defer after calling WithCancel/Cause
- Return error if node exited with non-zero exit status

## 0.3.0 - 2023-08-18

### Added

- Add Test ID regex filter support for the `list`, `run` and `report` commands
- Make Test ID a map of key/value pairs, and add "id" example to demonstrate
- Validate that Node IDs identify Nodes unambiguously
- Make output filenames configurable with a Go template (Test.OutputPrefix)
- Add `list` command to list tests
- Add `vet` command for checking CUE config
- Add support for setting node environment variables, and add "env" example
- Add support for setting DataFile in Test
- Add HTB quantums for all examples
- Add Report field to Test and default with EmitLog and SaveFiles
- Add All field to MessageFilter to easily accept all messages

### Fixed

- Fix System Runner not always waiting for IO to complete (e.g. short pcaps)
- Fix System Runner not always exiting until second interrupt
- Fix hang and improve errors on Go runtime failure (e.g. GOMEMLIMIT=bogus)
- Add sleeps to examples to make it more likely all packets are captured
- Add missing Schedule field when building node.Tree
- Return errors immediately on failed sets of sockopts

### Changed

- Require Go 1.21 in go.mod
- In System Runner, use new Command.Cancel func instead of interrupt goroutine
- Add `[0-9]` to allowable characters in flow IDs (`#Flow`)
- Limit flow IDs (`#Flow`) to 16 characters to reduce size of results
- Rename CUE template extension from `.ant` to `.cue.tmpl`

## 0.3.0-beta - 2022-10-13

### Added

- Runners with custom schedule
- Reports architecture, with templates for dual-axis goodput/OWD and FCT plots
- UDP flows with VBR support
- SSH support
- CUE configuration
- netns support
- Examples: iperf3, ratedrop, sceaqm, shortflows, tcpstream, vbrudp

### Changed

- Node v3 ("event loop")

## 0.2.0 - 2021-11-01

### Added

- FCT test

### Changed

- Node v2 ("channel heavy")

## 0.1.0 - 2021-05-01

### Added

- Initial prototype
- TCP goodput test
- Node v1 ("request-response")
