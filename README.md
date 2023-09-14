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

## Examples

| Example                     | Plot            |
| --------------------------- | --------------- |
| [fct](examples/fct/fct.cue.tmpl) | [fct](https://www.heistp.net/downloads/antler/examples/fct/fct.html) |
| [ratedrop](examples/ratedrop/ratedrop.cue) | [timeseries](https://www.heistp.net/downloads/antler/examples/ratedrop/timeseries.html) |
| [sceaqm](examples/sceaqm/sceaqm.cue) | [cake](https://www.heistp.net/downloads/antler/examples/sceaqm/cake_timeseries.html) / [cnq_cobalt](https://www.heistp.net/downloads/antler/examples/sceaqm/cnq_cobalt_timeseries.html) / [codel](https://www.heistp.net/downloads/antler/examples/sceaqm/codel_timeseries.html) / [pfifo](https://www.heistp.net/downloads/antler/examples/sceaqm/pfifo_timeseries.html) / [pie](https://www.heistp.net/downloads/antler/examples/sceaqm/pie_timeseries.html) / [cobalt](https://www.heistp.net/downloads/antler/examples/sceaqm/cobalt_timeseries.html) / [deltic](https://www.heistp.net/downloads/antler/examples/sceaqm/deltic_timeseries.html) |
| [tcpstream](examples/tcpstream/tcpstream.cue) | [timeseries](https://www.heistp.net/downloads/antler/examples/tcpstream/timeseries.html) |
| [vbrudp](examples/vbrudp/vbrudp.cue) | [timeseries](https://www.heistp.net/downloads/antler/examples/vbrudp/timeseries.html) |

To run the examples yourself, e.g.:
```
cd examples/tcpstream
sudo antler run
```

All configuration is in the .cue or .cue.tmpl files. Root access is required for
any examples that need to create network namespaces.

The antler binary must be in your PATH, or the full path must be specified.
Typically, you add ~/go/bin to your PATH so you can run binaries installed by
Go. *Note:* if using sudo and the `secure_path` option is set in /etc/sudoers,
either this must be added to that path, or additional configuration is required.

## Installation

### Using go install

1. Install [Go](https://go.dev/).
2. `go install github.com/heistp/antler@latest`
3. `make` (builds node binaries, installs antler command)

### Using git clone

1. Install [Go](https://go.dev/).
2. `cd`
3. `mkdir -p go/src/github.com/heistp`
4. `cd go/src/github.com/heistp`
5. `git clone https://github.com/heistp/antler`
6. `cd antler`
7. `make` (builds node binaries, installs antler command)

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

- add standard reports for each test:
  - node logs and system information
  - time series and FCT plots
  - tables of standard flow metrics: goodput, FCT, RTT distributions, etc
  - an HTML index of tests and results
- add support for sampling Linux socket stats via netlink (in C)
- implement timeouts, both for runners and the node control connection
- complete the SSH launcher, with sudo support, and add an example of its use
- for packet flows:
  - record replies and calculate RTT
  - detect lost and late (out of order) packets
- handle tests both with and without node-synchronized time
- add support for setting arbitary sockopts
- secure the servers for use on the Internet
- add compression support for System runner FileData output
- make UDP flood more efficient
- add an antler _init_ command to create a default project
- write documentation (in markdown)

### Version 0.4.0

#### Features

- implement incremental test runs using hard links
- move SCE examples into sce-tests repo and deploy to public server
- combine examples into one package, to share common setup
- build examples to a public server and remove from README
- include git tags in kernel and Antler versions, and record in results

#### Bugs

- improve poor error messages from CUE, especially under disjunctions

### Inbox

#### Features

- handle interrupts and context done in report command
- process pcaps to get retransmits, CE/SCE marks, TCP RTT or other stats
- add runner duration for use in timeouts and test duration estimation
- add test progress bar
- improve semantics for System.Stdout and Stderr
- add ability to save System Stdout directly to local file
- add ability to buffer System Stdout to a tmp file before sending as FileData
- add log command to emit LogEntry's to stdout
- enhance stream server protocol to ensure streams have completed
- implement flagForward optimization, and maybe invert it to flagProcess
- add support for simulating conversational stream protocols
- abort atomic writes when ResultWriter callers fail?
- show bandwidth for FCT distribution
- find a better way than unions to create interface implementations from CUE
- allow Go template syntax right in .cue files, instead of using .cue.tmpl files
- support multiple nodes in the same namespace
- add Antler to [CUE Unity](https://github.com/marketplace/cue-unity)
- support MacOS
- support FreeBSD

#### Refactoring

- convert longer funcs/methods to use explicit return values
- consistently document config in either config.cue or config structs
- replace use of chan any in conn
- replace type switch in node.Do

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice and
patience.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
