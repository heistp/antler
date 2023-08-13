# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Make Test ID a map of key/value pairs
- Make output filenames configurable with a Go template (Test.OutPathTemplate)
- Add `vet` command for checking CUE config
- Add support for setting node environment variables via Env CUE field
- Add support for setting DataFile in Test

### Fixed

- Fix System Runner not always waiting for IO to complete (e.g. short pcaps)
- Fix System Runner not always exiting until second interrupt

### Changed

- In System Runner, use new Command.Cancel func instead of interrupt goroutine

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
