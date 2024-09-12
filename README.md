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

## Why Antler?

In running tests with existing tools, I found that the job for congestion
control work tends to be time consuming and error prone, as it involves more
than just generating traffic and emitting stats, including:

* setting up and tearing down test environments
* orchestrating actions across multiple nodes
* running multiple tests with varied parameter combinations
* re-running only some tests while retaining prior results
* running external tools to gather pcaps or other data
* gathering results from multiple nodes into a single source of truth
* emitting results in different formats for consumption
* saving results non-destructively so prior work isn't lost
* making results available on the web
* configuring all of the above in a common way, to avoid mistakes

Antler is an attempt to address the above. The test environment is set up and
torn down before and after each test, preventing configuration mistakes and
"config bleed" from run to run. The test nodes are auto-installed and
uninstalled before and after each test, preventing version mismatch and
dependency problems. Tests are orchestrated using a hierarchy of serial and
parallel actions that can be coordinated over the control connections to each
node. Results, logs and data from all the nodes are gathered into a single data
stream, saved non-destructively, and processed in a report pipeline to produce
the output. Partial test runs allow re-running only some tests, while hard
linking results from prior runs so a complete result tree is always available.
Results may be published using an internal, embedded web server. Finally, all of
the configuration is done using [CUE](https://cuelang.org/), a data language
that helps avoid config mistakes and duplication.

## Features

### Tests

* auto-installed test nodes that run either locally or via ssh, and optionally
  in Linux network namespaces
* builtin traffic generator in Go:
  * support for tests using stream-oriented and packet-oriented protocols (for
    now, TCP and UDP)
  * configurable UDP packet release times and lengths, supporting anything from
    isochronous, to VBR or bursty traffic, or combinations in one flow
  * support for setting arbitrary sockopts, including CCA and the DS field
* configuration using [CUE](https://cuelang.org/), to support test parameter
  combinations, schema definition, data validation and config reuse
* configurable hierarchy of "runners", that may execute in serial or parallel
  across nodes, and with arbitrary scheduled timing (e.g. TCP flow introductions
  on an exponential distribution with lognormal lengths)
* incremental test runs to run only selected tests, and hard link the rest from
  prior results
* system runner for system commands, e.g. for setup, teardown, data collection
  such as pcaps, and mid-test config changes
* system information gathering from commands, files, environment variables and
  sysctls
* parallel execution of entire tests, with nested serial and parallel test runs

### Results/Reports

* time series and FCT plots using
  [Google Charts](https://developers.google.com/chart)
* plots/reports implemented with Go templates, which may eventually be
  written by users to target any plotting package
* optional result streaming during test (may be configured to deliver only some
  results, e.g. logs, but not pcaps)
* generation of index.html pages of tests
* embedded web server to serve results

## Status

As of version 0.6.0, many of the core features are implemented, along with some
basic tests and visualizations. The [Roadmap](#roadmap) shows future plans.
Overall, more work is needed to expand and improve the available plots,
stabilize the config and data formats, and support platforms other than Linux.

## Installation

1. Install [Go](https://go.dev/dl) (1.21 or later required).
2. `cd`
3. `mkdir -p go/src/github.com/heistp`
4. `cd go/src/github.com/heistp`
5. `git clone https://github.com/heistp/antler`
6. `cd antler`
7. `make` (builds node binaries, installs antler command)

To run antler, the binary must be in your PATH, or the full path must be
specified. Typically, you add ~/go/bin to your PATH so you can run binaries
installed by Go. *Note:* if using sudo and the `secure_path` option is set in
/etc/sudoers, either this must be added to that path, or additional
configuration is required.

## Examples

The examples output is available online 
[here](https://www.heistp.net/antler/examples/latest), where you can view the
HTML plots and log files. A few samples from that directory:

* [CUBIC vs BBRv1 through CoDel, with VBR UDP](https://www.heistp.net/antler/examples/latest/vbrudp_timeseries.html)
* [CUBIC vs BBRv1 FCT Competition](https://www.heistp.net/antler/examples/latest/fct_fct.html)
* [System Info](https://www.heistp.net/antler/examples/latest/iperf3_sysinfo_antler.html)

To run the examples yourself (root required for network namespaces):
```
cd examples
sudo antler run
```

All configuration is in the .cue or .cue.tmpl files, and the output is written
to the results directory.

## Documentation

Antler is currently documented through the [examples](examples), and the
comments in [config.cue](config.cue). Antler is configured using
[CUE](https://cuelang.org/), so it helps to get familiar with the language, but
for simple tests, it may be enough to just follow the examples.

## UDP Latency Accuracy Limits

The node and its builtin traffic generators are written in
[Go](https://go.dev/). This comes with some system call overhead and scheduling
jitter, which reduces the accuracy of the UDP latency results somewhat relative
to C/C++, or better yet timings obtained from the kernel or network. The
following comparison between ping and [irtt](https://github.com/heistp/irtt)
gives some idea (note the log scale on the vertical axis):

![Ping vs IRTT](/doc/img/ping-vs-irtt.svg "Ping vs IRTT")

While the UDP results are still useful for tests at most Internet RTTs, if
microsecond level accuracy is required, external tools should be invoked using
the System runner, or the times may be interpreted from pcaps instead. In the
future, either the traffic generation or the entire node may be rewritten in
another language, if required.

## Roadmap

### Version 1.0.0

- undergo security audit
- secure servers for use on the Internet
- enhance stream server protocol to ensure streams have completed
- add runner duration and use that to implement timeouts
- add an antler _init_ command to create a default project
- write documentation (in markdown)

### Inbox

#### Features

- try new evaluator in CUE 0.10 to improve performance during config parsing
- improve performance of low interval / UDP flood recording and plotting
- complete overhaul of configuration mechanism?
- implement traffic generator in C (or rewrite node in Rust)
- allow writing custom Go templates to generate any plot/report output
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
- figure out why default for #EmitSysInfo:To doesn't work (default-default)

## Thanks

A kind thanks to sponsors:

* **NLNet** and *NGI0 Core*
* **NGI Pointer**
* **RIPE NCC**

and to Jonathan Morton and Rodney Grimes for advice.

![NGI SCE Sticker](/doc/img/ngi-sce-sticker-200x230.png "NGI SCE Sticker")
