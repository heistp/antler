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

Antler v1.0 has some useful
[features](https://github.com/heistp/antler/wiki/#features), and has ungergone a
security audit, so it should be safe to use.

It is important to understand Antler's
[caveats](https://github.com/heistp/antler/wiki/#caveats).  In particular, the
use of [CUE](https://cuelang.org/) has been a mixed bag, and the visualizations
in Google Charts could be more flexible than they are.  Future versions of
Antler may replace the configuration and/or visualization mechanisms.

## Roadmap

### Inbox

The Inbox is a collection area for future tasks.

#### Features

- Research alternative configuration mechanisms
- Improve flexibility of visualizations (maybe allow custom Go templates)
- Add alternative plotting library to Google Charts
- Implement node or at least traffic generator in C or Rust
- Process pcaps to get retransmits, CE/SCE marks, TCP RTT or other stats
- Merge system info and logs into plots
- Improve performance of low interval / UDP flood recording and plotting
- Add results command to list results
- Add log command to emit LogEntry's to stdout
- Add rm command to remove result and update latest symlink
- Add ability to save System Stdout directly to local file
- Add node-side compression support for System runner FileData output
- Add ability to buffer System Stdout to a tmp file before sending as FileData
- Add admin web UI to run a package of tests
- Handle tests both with and without node-synchronized time
- Add test progress bar
- Implement flagForward optimization, and maybe invert it to flagProcess
- Add support for simulating conversational stream protocols
- Support FreeBSD
- Support MacOS

#### Refactoring

- Convert longer funcs/methods to use explicit return values
- Reconsider allowing empty Runs (see NOTE in run.go)
- Consistently document config in config.cue, with minimal doc in structs
- Replace use of chan any in conn
- Improve semantics for System.Stdout and Stderr
- Find a better way than unions to create interface implementations from CUE
- Consider moving all FileData to gob, for consistency with encoding

## Thanks

A kind thanks to sponsors:

* **NLNet** and **NGI0 Core**
* **NGI Pointer**
* **RIPE NCC**

and to Jonathan Morton and Rodney Grimes for advice.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")
