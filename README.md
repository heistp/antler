# Antler

Antler (Active Network Tester of Load Et Response) is a network testing tool,
mainly for congestion control and related work.

## Features

* support for stream-oriented and packet-oriented protocols (for now, TCP and
  UDP)
* UDP flows may be configured with arbitrary packet release times and lengths,
  supporting isochronous to VBR UDP flows
* control connection to all test hosts via auto-installed node, which may be
  run either locally or via ssh, and optionally in Linux network namespaces
* test orchestration across nodes with a configurable hierarchy of serial and
  parallel runners
* system runner allows execution of arbitrary system commands (e.g. for setup,
  teardown, mid-test config changes or data collection)
* runners may be scheduled using arbitrary timings and parameters (e.g. TCP
  flow introductions on an exponential distribution with lognormal lengths)
* optional streaming of results during test
* plots/reports using Go templates, with included templates for time series and
  FCT plots using Google Charts
* flexible configuration using [CUE](https://cuelang.org/)

## Status / Known Issues

At version 0.3, some basic tests and visualizations are working. More work is
needed to complete critical features, stabilize the config and data formats, and
support platforms other than Linux.

The initial focus has been on local netns tests. More work is required on
physical network tests, and handling nodes without synchronized time.

## Installation

## Quick Start

## Roadmap

### Features

- record packet replies and calculate RTT for packet flows
- detect lost and late (out-of-order) packets in packet flows, and flag with
  altered symbology in time series plot
- add an HTML index of tests and results
- gather node system information for inclusion in reports
- add SaveLog reporter that sorts logs by time
- add support for sampling Linux socket stats via netlink
  (like [cgmon](https://github.com/heistp/cgmon))
- add support for setting arbitary sockopts
- implement flagForward optimization, and maybe invert it to flagProcess
- protect public servers with three-way handshake for packet protocols and
  authentication for stream protocols
- add compression support for System runner FileData output
- support MacOS and FreeBSD

### Bugs

- improve poor error messages from CUE for syntax errors under disjunctions
- figure out why packets from tcpdump may be lost without a one-second
  post-test sleep (buffering? shouldn't SIGINT flush that?)

### Architecture

- find a better way than hand-coded unions to create interface types from CUE
- handle timeouts consistently for runners and the node control connection
- see if it's practical to move CUE constraints to Go
- design some way to implement incremental test runs, perhaps via hard links
- reduce use of type switches for data stream?

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
