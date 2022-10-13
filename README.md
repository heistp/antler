# Antler

Antler (Active Network Tester of Load Et Response) is a network testing tool
intended for congestion control and related work.

## Features

* support for stream-oriented and packet-oriented protocols (for now, TCP and
  UDP)
* auto-installed test nodes that run either locally or via ssh, and optionally
  in Linux network namespaces
* configurable hierarchy of "runners", that may execute in serial or parallel
  across nodes
* runner scheduling with arbitrary timings (e.g. TCP flow introductions on an
  exponential distribution with lognormal lengths)
* configurable UDP packet release times and lengths, supporting isochronous
  and VBR UDP flows
* system runner runs arbitrary system commands, e.g. for setup, teardown, data
  collection, and mid-test config changes
* parallel test execution, with nested serial and parallel runs
* result streaming during test (may be enabled, disabled or configured to
  deliver only some results, e.g. just logs)
* plots/reports using Go templates, with included templates for time series and
  FCT plots using [Google Charts](https://developers.google.com/chart)
* configuration using [CUE](https://cuelang.org/), to support test parameter
  combinations, config schema definition and data validation

## Examples

| Example                     | Plot            |
| --------------------------- | --------------- |
| [fct](examples/fct/fct.cue) | [fct](https://raw.githubusercontent.com/heistp/antler/examples/fct/fct.html) |
| [ratedrop](examples/ratedrop/ratedrop.cue) | [timeseries](https://raw.githubusercontent.com/heistp/antler/examples/ratedrop/timeseries.html) |
| [sceaqm](examples/sceaqm/sceaqm.cue) | [cake](https://raw.githubusercontent.com/heistp/antler/examples/sceaqm/cake_timeseries.html) / [cnq_cobalt](https://raw.githubusercontent.com/heistp/antler/examples/sceaqm/cnq_cobalt_timeseries.html) / [codel](https://raw.githubusercontent.com/heistp/antler/examples/sceaqm/codel_timeseries.html) / [pfifo](https://raw.githubusercontent.com/heistp/antler/examples/sceaqm/pfifo_timeseries.html) / [pie](https://raw.githubusercontent.com/heistp/antler/examples/sceaqm/pie_timeseries.html) |
| [tcpstream](examples/tcpstream/tcpstream.cue) | [timeseries](https://raw.githubusercontent.com/heistp/antler/examples/tcpstream/tcpstream.html) |
| [vbrudp](examples/vbrudp/vbrudp.cue) | [timeseries](https://raw.githubusercontent.com/heistp/antler/examples/vbrudp/vbrudp.html) |

To run the examples yourself, e.g.:
```
cd examples/tcpstream
sudo antler run
```

Root access is needed to create network namespaces.

The antler binary must be in your PATH, or the full path must be specified.
Typically, you add ~/go/bin to your PATH so you can run binaries installed by
Go. If using sudo and the `secure_path` option is set in /etc/sudoers, either
this must be removed, or additional configuration is required.

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

## Status

At version 0.3, some basic tests and visualizations are working. More work is
needed on testing functionality, stabilizing the config and data formats, and
supporting platforms other than Linux.

The initial focus has been on local netns tests. More work is required for tests
on physical networks, and handling nodes without synchronized time.

## Todo / Roadmap

### Features

- record packet replies and calculate RTT for packet flows
- detect lost and late (out-of-order) packets in packet flows, and flag with
  altered symbology in time series plot
- add an HTML index of tests and results
- add report type with standard output for each test:
  - node logs and system information
  - descriptive details for test
  - time series and FCT plots, with navigation instructions
- add SaveLog reporter that sorts logs by time
- add support for sampling Linux socket stats via netlink
  (like [cgmon](https://github.com/heistp/cgmon))
- add support for setting arbitary sockopts
- implement flagForward optimization, and maybe invert it to flagProcess
- protect public servers with three-way handshake for packet protocols and
  simple authentication for stream protocols
- add compression support for System runner FileData output
- add more plotting templates, e.g. for plotly, Gnuplot and xplot
- support MacOS
- support FreeBSD

### Bugs

- improve poor error messages from CUE for syntax errors under disjunctions
- return error immediately when CCA not found
- cancel Parallel test set when any one of them fails
- figure out why packets from tcpdump may be lost without a one-second
  post-test sleep (buffering? but shouldn't SIGINT flush that?)

### Architecture

- find a better way than unions to create interface implementations from CUE
- handle timeouts consistently, both for runners and the node control connection
- see if it's practical to move CUE schema from config.cue into Go
- share more CUE code for rig setups and between packages
- design some way to implement incremental test runs, perhaps via hard links
- reduce use of type switches for result data stream?

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
