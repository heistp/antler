# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to
[Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## 0.5.0 - 2024-06-27

### Added

- Implement multi-test reports via MultiReport in config.cue
- Add generation of index.html pages with tests and links to results
- Add table of flow metrics (goodput, FCT, etc.) to timeseries and fct plots

### Changed

- Replace the TestRun hierarchy in the config with a flat list of Tests
- Rename Test.During to DuringDefault, and TestRun.Report to Test.During
- Remove default ID for single Tests
- Change node log files to have .txt extension so they open in browser
- Rename Test.ResultPrefix/X to Test.Path

### Fixed

- Pull in x/net 0.26 to fix CVE-2023-45288 and make Dependabot happy
- Make the Analyze reporter concurrent-safe
- Log background command exits as exited command instead of error
- Use filepath.Separator instead of hardcoded slashes
- Make TestID.Match return true for a zero ID pattern

## 0.4.0 - 2023-12-20

### Added

- Implement incremental test runs
- Implement transparent result file encoding (compression/decompression)
- Implement pipelined reports (`TestRun.Report`, `Test.During`, `Test.Report`)
- Turn Analyze into a report and add it to examples that need it
- Implement log sorting
- Write test results non-destructively and atomically
- Validate ResultPrefixes are unique
- Add embedded web server to serve results (`server` command)
- Implement system information (`SysInfo`), with source code tags, commands,
  files, environment variables and sysctls
- Implement generic socket options for TCP and UDP
- Add support for setting the DS field (ToS/Traffic Class)

### Changed

- Replace node.Control with context.WithCancelCause from Go 1.20
- Change usages of interface{} to the `any` alias from Go 1.18
- Remove conn.Close and simplify connection closure
- Rename node.NodeID to node.ID to reduce stutter
- Rename Test.OutputPrefix to Test.ResultPrefix
- Make default Test ID `{"test": "single"}`
- Move SCE tests to [sce-tests](https://github.com/heistp/sce-tests) repo
- Combine examples into one CUE package and deploy to public server

### Fixed

- Fix hang after Go runtime panic in node
- Propagate parent context to node.runs goroutine
- Fix panic in FCT analysis when no data points are available
- Fix one second cancellation delay for Stream tests (check Context in receive)
- Consistently cancel Contexts in defer after calling WithCancel/Cause
- Return error if node exited with non-zero exit status
- Fix that no result was saved after setting Encode Destructive field to true

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
