# Antler

Antler is a tool for network and congestion control testing. The name stands for
**A**ctive **N**etwork **T**ester of **L**oad & **R**esponse, where '&' ==
**E**t. :)

## Introduction

Antler can be used to set up and tear down test environments, coordinate traffic
flows across multiple nodes, gather data using external tools like tcpdump, and
generate reports and plots from the results. It grew out of testing needs for
[SCE](https://datatracker.ietf.org/doc/draft-morton-tsvwg-sce/), and related
congestion control projects in the IETF.

## Features

* support for tests using stream-oriented and packet-oriented protocols (for
  now, TCP and UDP)
* auto-installed test nodes that run either locally or via ssh, and optionally
  in Linux network namespaces
* configurable hierarchy of "runners", that may execute in serial or parallel
  across nodes
* runner scheduling with arbitrary timing (e.g. TCP flow introductions on an
  exponential distribution with lognormal lengths)
* configurable UDP packet release times and lengths, supporting from isochronous
  to VBR or bursty traffic, or combinations in one flow
* system runner for system commands, e.g. for setup, teardown, data collection,
  and mid-test config changes
* parallel execution of entire tests, with nested serial and parallel test runs
* result streaming during test (may be enabled, disabled or configured to
  deliver only some results, e.g. just logs, but not pcaps)
* plots/reports using Go templates, with included templates for time series and
  FCT plots using [Google Charts](https://developers.google.com/chart)
* configuration using [CUE](https://cuelang.org/), to support test parameter
  combinations, config schema definition, data validation and config reuse
* embedded web server to serve results

## Installation

1. Install [Go](https://go.dev/dl) (1.21 or later required).
2. `cd`
3. `mkdir -p go/src/github.com/heistp`
4. `cd go/src/github.com/heistp`
5. `git clone https://github.com/heistp/antler`
6. `cd antler`
7. `make` (builds node binaries, installs antler command)

The antler binary must be in your PATH, or the full path must be specified.
Typically, you add ~/go/bin to your PATH so you can run binaries installed by
Go. *Note:* if using sudo and the `secure_path` option is set in /etc/sudoers,
either this must be added to that path, or additional configuration is required.

## Examples

The examples output is available online 
[here](https://www.heistp.net/antler/examples/latest), where you can view the
HTML plots and log files.

To run the examples yourself (root required for network namespaces):
```
cd examples
sudo antler run
```

All configuration is in the .cue or .cue.tmpl files, and the output is written
to the results directory.

## Documentation

Antler is documented through the [examples](examples), and the comments in
[config.cue](config.cue). Antler is configured using
[CUE](https://cuelang.org/), so it helps to get familiar with the language, but
for simple tests, it may be enought to just follow the examples.

## Status

As of version 0.3.0, the node is working, along with some basic tests and
visualizations. The [Roadmap](#roadmap) shows future plans. Overall, more work
is needed on the tests and visualizations, stabilizing the config and data
formats, and supporting platforms other than Linux.

## Roadmap

### Version 1.0.0

- undergo security audit
- secure servers for use on the Internet
- enhance stream server protocol to ensure streams have completed
- add runner duration and use that to implement timeouts
- add an antler _init_ command to create a default project
- write documentation (in markdown)

### Version 0.6.0

- add support for sampling Linux socket stats via netlink (in C)
- for packet flows:
  - record replies and calculate RTT
  - detect lost and late (out of order) packets
- complete the SSH launcher, with sudo support, and add an example of its use

### Version 0.5.0

- add an HTML index of tests and results
- add standard reports for each test:
  - time series and FCT plots
  - table of standard flow metrics, including goodput, FCT and data transferred
  - node logs
  - system information
  - git tags
- add admin web UI to run a package of tests

### Inbox

#### Features

- add rm command to remove result and update latest symlink
- add ls command to list results
- make UDP flood more efficient
- add node-side compression support for System runner FileData output
- handle tests both with and without node-synchronized time
- process pcaps to get retransmits, CE/SCE marks, TCP RTT or other stats
- add test progress bar
- add ability to save System Stdout directly to local file
- add ability to buffer System Stdout to a tmp file before sending as FileData
- add log command to emit LogEntry's to stdout
- implement flagForward optimization, and maybe invert it to flagProcess
- add support for simulating conversational stream protocols
- support multiple nodes in the same namespace
- add Antler to [CUE Unity](https://github.com/marketplace/cue-unity)
- support MacOS
- support FreeBSD

#### Refactoring

- convert longer funcs/methods to use explicit return values
- consistently document config in config.cue, with minimal doc in structs
- replace use of chan any in conn
- improve semantics for System.Stdout and Stderr
- find a better way than unions to create interface implementations from CUE
- consider moving all FileData to gob, for consistency with encoding

#### Bugs

- improve poor error messages from CUE, especially under disjunctions

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice and
patience.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
