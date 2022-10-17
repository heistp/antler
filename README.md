# Antler

Antler is an open-source network testing tool intended for congestion control
and related work. The name stands for **A**ctive **N**etwork **T**ester of
**L**oad & **R**esponse (with '&' derived from the ligature of **E**t :).

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
| [fct](examples/fct/fct.cue) | [fct](https://www.heistp.net/downloads/antler/examples/fct/fct.html) |
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
either this must be removed, or additional configuration is required.

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
visualizations. The [Roadmap](#roadmap) lists what should be completed before
upcoming releases.

Long term, more work is needed on functionality of the tests themselves,
stabilizing the config and data formats, and supporting platforms other than
Linux.

## Roadmap

### Version 0.3.0

- add README template and sceaqm links for bursty vs non-bursty results
- add support for sampling Linux socket stats via netlink
  (like [cgmon](https://github.com/heistp/cgmon))
- test the SSH launcher and add an example of its use
- add sudo support to the SSH launcher, instead of requiring root for netns
- record packet replies and calculate RTT for packet flows
- handle node data properly both with and without node-synchronized time
- detect lost and late (out-of-order) packets in packet flows, and flag with
  altered symbology in time series plot
- ChartsTimeSeries: automatically add one or both Y axes based on the data
  series present in the Test
- rename EmitLog reporter to Log, and add sorting by time for saved log files
- verify CUE disjunctions are used properly for union types

### Future, Maybe

#### Features

- add an HTML index of tests and results
- auto-detect node platforms
- add report type with standard output for each test:
  - node logs and system information
  - descriptive details for test
  - time series and FCT plots, with navigation instructions
  - tables of standard flow metrics: goodput, FCT, RTT distributions, etc
- add support for setting arbitary sockopts
- add configuration to simulate conversational stream protocols
- implement flagForward optimization, and maybe invert it to flagProcess
- protect public servers with three-way handshake for packet protocols and
  simple authentication for stream protocols
- add compression support for System runner FileData output
- add more plotting templates, e.g. for plotly, Gnuplot and xplot
- implement traffic generators in C, for performance
- write full documentation
- support MacOS
- support FreeBSD

#### Bugs

- improve poor error messages from CUE for syntax errors under disjunctions
- return errors immediately on failed sets of CCA / sockopts, instead of
  waiting until the end of the test
- figure out why packets from tcpdump may be lost without a one-second post-test
  sleep (maybe buffering, but shouldn't SIGINT flush that? is this a result of
  the network namespaces teardown issue below?)
- network namespaces may be deleted before runners have completed, for example
  if a middlebox is canceled and terminated before the endpoints have completed-
  possibly add an additional node state during cancellation to handle this

#### Architecture

- find a better way than unions to create interface implementations from CUE
- handle timeouts consistently, both for runners and the node control connection
- add antler init command to save config schema and defaults?
- share more CUE code in examples, especially for netns rig setups
- design some way to implement incremental test runs, perhaps using hard links

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice and
patience.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
