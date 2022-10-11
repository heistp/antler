# Antler

Active Network Tester of Load Et Response

Antler is a network testing tool for congestion control and related work.

At version 0.3, some basic tests and visualizations are working. More work is
needed to complete key features, stabilize the config and data formats and
support platforms other than Linux.

## Features

* control connection to all test hosts via auto-installed node that may be run
  locally, in a Linux network namespace or via ssh
* test orchestration via a configurable hierarchy of serial and parallel runners
  which can execute across nodes
* support for stream-oriented and packet-oriented protocols (for now, TCP and
  UDP)
* per-stream configurable CCA (Congestion Control Algorithm)
* runners may be scheduled using arbitrary timings and parameters, e.g. TCP
  flow introductions on an exponential distribution with lognormal lengths
* packet-oriented flows may be configured with arbitrary packet release times
  and lengths, e.g. from isochronous to VBR UDP flows
* output redirection from node system commands, e.g. to gather pcaps from
  multiple nodes
* plots and reports using Go templates, with included templates for time series
  and FCT plots using Google Charts
* flexible configuration using [CUE](https://cuelang.org/)

## Installation

## Quick Start

## Roadmap

### Features

- add an HTML index of tests and results
- calculate RTT for packet flows
- detect lost and late (out-of-order) packets in packet flows, and flag with
  altered symbology in time series plot
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

- improve poor error messages due to config problems, particularly under CUE
  disjunctions
- figure out why packets from tcpdump may be lost without a one-second
  post-test sleep (buffering? shouldn't SIGINT flush that?)

### Architecture

- find a better way than hand-coded unions to create interface types from CUE
- see if it's practical to move CUE constraints to Go
- design some way to implement incremental test runs, perhaps via hard links
- reduce use of type switches for data stream?

## Thanks

Thanks to sponsors, and Jonathan Morton and Rodney Grimes for advice.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")

**RIPE NCC**
