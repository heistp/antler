# Antler

Antler is a tool for network and congestion control testing. The name stands for
**A**ctive **N**etwork **T**ester of **L**oad & **R**esponse, where '&' ==
**E**t. :)

Antler can be used to set up and tear down test environments, coordinate traffic
flows across multiple nodes, gather data using external tools like tcpdump, and
generate reports and plots from the results.  It grew out of testing needs for
[SCE](https://datatracker.ietf.org/doc/draft-morton-tsvwg-sce/), and related
congestion control projects in the IETF.

## Examples

The examples output is available online 
[here](https://www.heistp.net/antler/examples/latest), where you can view the
HTML plots and log files. A few samples from that directory:

* [CUBIC vs BBRv1 through CoDel, with VBR UDP](https://www.heistp.net/antler/examples/latest/vbrudp_timeseries.html)
* [CUBIC vs BBRv1 FCT Competition](https://www.heistp.net/antler/examples/latest/fct_fct.html)
* [System Info](https://www.heistp.net/antler/examples/latest/iperf3_sysinfo_antler.html)

To run the examples yourself, [install Antler](https://github.com/heistp/antler/wiki/Getting-Started#installation), then in the `examples` directory:
```
sudo antler run
```

All configuration is in the `.cue` or `.cue.tmpl` files, and the output is
written to the `results` directory.

## Documentation

Documentation for Antler is available in the
[Wiki](https://github.com/heistp/antler/wiki).

## Status

As of version 0.7.1, many of the core
[features](https://github.com/heistp/antler/wiki/#features) are implemented,
along with some basic tests and visualizations.  More work on security is
planned for 1.0.0, but Antler should be safe to use in controlled environments.

It is important to understand Antler's
[caveats](https://github.com/heistp/antler/wiki/#caveats).  In particular, the
use of [CUE](https://cuelang.org/) has been a mixed bag, and the visualizations
in Google Charts could be more flexible than they are.  Future versions of
Antler may replace the configuration and/or visualization mechanisms.

## Roadmap

### Version 1.0.0

- add test traffic header encryption
- add netns support with minimal sudo requirements
- undergo security audit

### Inbox

The Inbox is a collection area for tasks that may (or may not) happen in the
future.

#### Features

- switch to a different configuration mechanism
- improve flexibility of visualizations (maybe allow custom Go templates)
- add plotting library alternative to Google Charts
- improve performance of linking prior results in incremental builds
- improve performance of low interval / UDP flood recording and plotting
- implement traffic generator in C (or rewrite node in Rust)
- merge system info and logs into plots
- add rm command to remove result and update latest symlink
- add ls command to list results
- add admin web UI to run a package of tests
- add node-side compression support for System runner FileData output
- handle tests both with and without node-synchronized time
- process pcaps to get retransmits, CE/SCE marks, TCP RTT or other stats
- add test progress bar
- add ability to save System Stdout directly to local file
- add ability to buffer System Stdout to a tmp file before sending as FileData
- add log command to emit LogEntry's to stdout
- implement flagForward optimization, and maybe invert it to flagProcess
- add support for simulating conversational stream protocols
- support FreeBSD
- support MacOS

#### Refactoring

- reconsider allowing empty Runs (see NOTE in run.go)
- convert longer funcs/methods to use explicit return values
- consistently document config in config.cue, with minimal doc in structs
- replace use of chan any in conn
- improve semantics for System.Stdout and Stderr
- find a better way than unions to create interface implementations from CUE
- consider moving all FileData to gob, for consistency with encoding

## Thanks

A kind thanks to sponsors:

* **NLNet** and **NGI0 Core**
* **NGI Pointer**
* **RIPE NCC**

and to Jonathan Morton and Rodney Grimes for advice.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")
