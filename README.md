# Antler

Antler is an open-source network testing tool intended for congestion control
and related work. The name stands for **A**ctive **N**etwork **T**ester of
**L**oad & **R**esponse, where '&' == **E**t. :)

## Introduction

Antler grew out of testing needs that arose during work on
[SCE](https://datatracker.ietf.org/doc/draft-morton-tsvwg-sce/), and related
congestion control projects in the IETF. It can be used to set up and tear down
test environments, coordinate traffic flows across multiple nodes, gather data
using external tools like tcpdump, and generate reports and plots from the
results.

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
* parallel test execution, with nested serial and parallel runs
* result streaming during test (may be enabled, disabled or configured to
  deliver only some results, e.g. just logs)
* plots/reports using Go templates, with included templates for time series and
  FCT plots using [Google Charts](https://developers.google.com/chart)
* configuration using [CUE](https://cuelang.org/), to support test parameter
  combinations, config schema definition, data validation and config reuse

## Examples

| Example                     | Plot            |
| --------------------------- | --------------- |
| [fct](examples/fct/fct.ant) | [fct](https://www.heistp.net/downloads/antler/examples/fct/fct.html) |
| [ratedrop](examples/ratedrop/ratedrop.cue) | [timeseries](https://www.heistp.net/downloads/antler/examples/ratedrop/timeseries.html) |
| [sceaqm](examples/sceaqm/sceaqm.cue) | [cake](https://www.heistp.net/downloads/antler/examples/sceaqm/cake_timeseries.html) / [cnq_cobalt](https://www.heistp.net/downloads/antler/examples/sceaqm/cnq_cobalt_timeseries.html) / [codel](https://www.heistp.net/downloads/antler/examples/sceaqm/codel_timeseries.html) / [pfifo](https://www.heistp.net/downloads/antler/examples/sceaqm/pfifo_timeseries.html) / [pie](https://www.heistp.net/downloads/antler/examples/sceaqm/pie_timeseries.html) / [cobalt](https://www.heistp.net/downloads/antler/examples/sceaqm/cobalt_timeseries.html) / [deltic](https://www.heistp.net/downloads/antler/examples/sceaqm/deltic_timeseries.html) |
| [tcpstream](examples/tcpstream/tcpstream.cue) | [timeseries](https://www.heistp.net/downloads/antler/examples/tcpstream/timeseries.html) |
| [vbrudp](examples/vbrudp/vbrudp.cue) | [timeseries](https://www.heistp.net/downloads/antler/examples/vbrudp/timeseries.html) |

To run the examples yourself, e.g.:
```
cd examples/tcpstream
sudo antler run
```

Root access is required to create network namespaces.

The antler binary must be in your PATH, or the full path must be specified.
Typically, you add ~/go/bin to your PATH so you can run binaries installed by
Go. *Note:* if using sudo and the `secure_path` option is set in /etc/sudoers,
either this must be added to that path, or additional configuration is required.

All configuration is in the .cue file. After running the examples, you'll 
typically have gob files, pcaps and an HTML plot.

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

At present, Antler is documented through the [examples](examples), and the
comments in [config.cue](config.cue). Antler is configured using
[CUE](https://cuelang.org/), so it helps to get familiar with the language at a
basic level, though for simple tests it may be sufficient to just follow the
examples.

## Status

As of version 0.3.0-beta, the node is working, along with some basic tests and
visualizations. The [Roadmap](#roadmap) shows future plans.

TODO

More work is needed on the tests and visualizations, stabilizing the config and
data formats, and supporting platforms other than Linux.

## Roadmap

### Version 1.0.0

- add standard reports for each test:
  - node logs and system information
  - time series and FCT plots
  - tables of standard flow metrics: goodput, FCT, RTT distributions, etc
  - an HTML index of tests and results
- add support for sampling Linux socket stats via netlink (in C)
- implement incremental test runs using hard links
- implement timeouts, both for runners and the node control connection
- complete the SSH launcher, with sudo support, and add an example of its use
- for packet flows:
  - record replies and calculate RTT
  - detect lost and late (out of order) packets
- handle tests both with and without node-synchronized time
- add support for setting arbitary sockopts
- rename EmitLog Reporter to Log, and add sorting by time for saved log files
- optionally secure the servers for public use using a three-way handshake
- add compression support for System runner FileData output
- make UDP flood more efficient
- add an antler _init_ command to create a default project
- write documentation (in markdown)

### Version 0.3.0

#### Features

- refactor examples to share common setup
  - add explicit htb quantum

#### Bugs

- stream everything by default in root node
- test for heap retention when streaming FileData
- reconsider semantics for System.Stdout and Stderr
- return error when trying to write FileData to absolute paths
- improve error handling with bad Go runtime settings (e.g. GOMEMLIMIT=bad)
- fix that tests may not be canceled until the second interrupt
- return errors immediately on failed sets of CCA / sockopts, instead of
  waiting until the end of the test
- figure out why packets from tcpdump may be lost without a one-second post-test
  sleep (maybe buffering, but shouldn't SIGINT flush that? is this a result of
  the network namespaces teardown issue below?)
- network namespaces may be deleted before runners have completed, for example
  if a middlebox is canceled and terminated before the endpoints have completed-
  possibly add an additional node state during cancellation to handle this
- fix poor CUE error when Env length > max (Run.Test: field not allowed)

### Inbox

#### Features

- implement flagForward optimization, and maybe invert it to flagProcess
- add support for simulating conversational stream protocols
- consider adding more plotting templates, e.g. for plotly, Gnuplot and xplot
- implement traffic generators in C
- show bandwidth for FCT distribution
- add support for messages between nodes for runner coordination
- find a better way than unions to create interface implementations from CUE
- share more CUE code in examples, especially for netns rig setups
- support MacOS
- support FreeBSD

#### Bugs

- improve poor error messages from CUE for syntax errors under disjunctions, and
  verify disjunctions are used properly for union types

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice and
patience.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
