// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

/*
#cgo CFLAGS: -O2 -Wall

#include "sockdiag.h"
*/
import "C"

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/heistp/antler/node/metric"
	"golang.org/x/sys/unix"
)

// sockdiag gathers socket statistics using the sock_diag(7) netlink subsystem
// on Linux. A sampler goroutine is created for each unique sampling interval,
// as a basic means of timer coalescing. This avoids the need to create a
// sampling goroutine for each flow. It is possible, though wasteful, to sample
// the same socket address at multiple different intervals.
type sockdiag struct {
	ev      chan event
	sampler map[time.Duration]*sockdiagSampler
	mtx     sync.Mutex
	cxl     chan struct{}
}

// newSockdiag returns a new sockdiag.
func newSockdiag(ev chan event) *sockdiag {
	return &sockdiag{
		ev,
		make(map[time.Duration]*sockdiagSampler),
		sync.Mutex{},
		make(chan struct{}),
	}
}

// Add adds the given socket address for TCPInfo sampling at the given interval.
// Since Flow corresponds to the 5-tuple for TCP, the Flow in the given info
// must uniquely identify the src and dst socket addresses in addr.
func (d *sockdiag) Add(addr sockAddr, info tcpFlowInfo, interval time.Duration) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var s *sockdiagSampler
	if s = d.sampler[interval]; s == nil {
		s = newSockdiagSampler(d.ev, interval)
		d.sampler[interval] = s
	}
	s.Add(addr, info)
}

// Remove stops sampling for the given sock address, at the given interval.
func (d *sockdiag) Remove(addr sockAddr, interval time.Duration) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var s *sockdiagSampler
	if s = d.sampler[interval]; s == nil {
		return
	}
	if s.Remove(addr) {
		s.Stop()
		delete(d.sampler, interval)
	}
}

// Stops stops all samplers and waits for them to complete.
func (d *sockdiag) Stop() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	for i, s := range d.sampler {
		s.Stop()
		delete(d.sampler, i)
	}
}

// sockdiagSampler samples socket statistics on a fixed interval, and sends
// TCPInfo's with the statistics to the node's event channel.
type sockdiagSampler struct {
	addr4    map[sockAddr4]tcpFlowInfo
	addr6    map[sockAddr6]tcpFlowInfo
	ev       chan event
	interval time.Duration
	mtx      sync.Mutex
	started  bool
	cxl      chan struct{}
	done     chan struct{}
}

// newSockdiagSampler returns a new sockdiagSampler that samples socket
// statistics on the given interval.
func newSockdiagSampler(ev chan event, interval time.Duration) *sockdiagSampler {
	return &sockdiagSampler{
		make(map[sockAddr4]tcpFlowInfo),
		make(map[sockAddr6]tcpFlowInfo),
		ev,
		interval,
		sync.Mutex{},
		false,
		make(chan struct{}),
		make(chan struct{}),
	}
}

// Add registers the given socket address to send TCPInfo for, with the given
// flow info. If this is the first address added, the sampling goroutine is
// started.
func (m *sockdiagSampler) Add(addr sockAddr, info tcpFlowInfo) {
	m.mtx.Lock()
	defer func() {
		if !m.started && !m.empty() {
			m.started = true
			go m.run()
		}
		m.mtx.Unlock()
	}()
	var a4 sockAddr4
	var ok bool
	if a4, ok = addr.To4(); ok {
		m.addr4[a4] = info
		return
	}
	var a6 sockAddr6
	if a6, ok = addr.To6(); ok {
		m.addr6[a6] = info
		return
	}
	panic(fmt.Sprintf("unknown IP version for address: %s", addr))
}

// tcpFlowInfo contains the flow and orientation information in TCPInfo.
type tcpFlowInfo struct {
	Flow     Flow
	Location Location
}

// empty returns true if no addresses are registered.
func (m *sockdiagSampler) empty() bool {
	return len(m.addr4) == 0 && len(m.addr6) == 0
}

// Remove unregisters the given ...
func (m *sockdiagSampler) Remove(addr sockAddr) (empty bool) {
	m.mtx.Lock()
	defer func() {
		empty = m.empty()
		m.mtx.Unlock()
	}()
	var a4 sockAddr4
	var ok bool
	if a4, ok = addr.To4(); ok {
		delete(m.addr4, a4)
		return
	}
	var a6 sockAddr6
	if a6, ok = addr.To6(); ok {
		delete(m.addr6, a6)
		return
	}
	panic(fmt.Sprintf("unknown IP version for address: %s", addr))
}

// run is the entry point for the sockdiagSampler goroutine.
func (m *sockdiagSampler) run() {
	defer close(m.done)
	t := time.NewTicker(m.interval)
	defer t.Stop()
	var e error
	defer func() {
		if e != nil {
			m.ev <- errorEvent{e, false}
		}
	}()
	var fd C.int
	if fd, e = C.sockdiag_open(); fd < 0 {
		return
	}
	defer C.sockdiag_close(fd)
	f := true
	var d bool
	for !d {
		select {
		case <-m.cxl:
			d = true
		case <-t.C:
			if f {
				f = false
				break
			}
			if e = m.sample(fd); e != nil {
				d = true
			}
		}
	}
}

// sample locks the sampler and calls sampleFamily for IPv4 and/or IPv6,
// according to which IP versions there are registered addresses for.
func (m *sockdiagSampler) sample(fd C.int) (err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if len(m.addr4) > 0 {
		if err = m.sampleFamily(fd, unix.AF_INET); err != nil {
			return
		}
	}
	if len(m.addr6) > 0 {
		err = m.sampleFamily(fd, unix.AF_INET6)
	}
	return
}

// sampleFamily uses netlink to get tcp_info arrays for the given socket family
// (AF_INET or AF_INET6), and sends TCPInfo's for each address registered with
// the sampler.
func (m *sockdiagSampler) sampleFamily(fd C.int, family C.uchar) (err error) {
	var cs C.struct_samples
	t0 := metric.Now()
	if _, err = C.sockdiag_sample(fd, family, &cs); err != nil {
		return
	}
	t := metric.Now()
	ss := (*[1 << 30]C.struct_sample)(unsafe.Pointer(cs.sample))[:cs.len:cs.len]
	for _, s := range ss {
		var ok bool
		var fi tcpFlowInfo
		if s.family == unix.AF_INET {
			fi, ok = m.addr4[sampleSockAddr4(s)]
		} else {
			fi, ok = m.addr6[sampleSockAddr6(s)]
		}
		if !ok {
			continue
		}
		m.ev <- newTCPInfo(fi, t, time.Duration(t-t0), s.info)
	}
	C.sockdiag_free_samples(&cs)
	return
}

// TCPInfo contains a subset of the socket statistics from Linux's tcp_info
// struct, defined in include/uapi/linux/tcp.h.
type TCPInfo struct {
	tcpFlowInfo

	// T is the relative time the corresponding tcp_info was received.
	T metric.RelativeTime

	// SampleTime is the elapsed time it took to get the tcp_info from the
	// kernel.
	SampleTime time.Duration

	// RTT is the round-trip time, from tcpi_rtt.
	RTT time.Duration

	// RTTVar is the round-trip time variance, from tcpi_rttvar.
	RTTVar time.Duration

	// TotalRetransmits is the total number of retransmits, from
	// tcpi_total_retrans.
	TotalRetransmits int

	// DeliveryRate is the packet delivery rate from the kernel pacing stats,
	// from tcpi_delivery_rate.
	DeliveryRate metric.Bitrate

	// PacingRate is the packet pacing rate from the kernel pacing stats, from
	// tcpi_pacing_rate.
	PacingRate metric.Bitrate

	// SendCwnd is the send congestion window, in units of MSS, from
	// tcpi_snd_cwnd.
	SendCwnd int

	// SendMSS is the send maximum segment size, from tcpi_snd_mss.
	SendMSS metric.Bytes
}

// newTCPInfo returns a new TCPInfo from a sockdiag sample.
func newTCPInfo(fi tcpFlowInfo, t metric.RelativeTime, st time.Duration,
	ti C.struct_tcp_info) TCPInfo {
	return TCPInfo{
		fi,
		t,
		st,
		time.Duration(time.Duration(ti.tcpi_rtt) * time.Microsecond),
		time.Duration(time.Duration(ti.tcpi_rttvar) * time.Microsecond),
		int(ti.tcpi_total_retrans),
		metric.Bitrate(ti.tcpi_delivery_rate * 8),
		metric.Bitrate(ti.tcpi_pacing_rate * 8),
		int(ti.tcpi_snd_cwnd),
		metric.Bytes(ti.tcpi_snd_mss),
	}
}

// init registers TCPInfo with the gob encoder
func init() {
	gob.Register(TCPInfo{})
}

// flags implements message
func (TCPInfo) flags() flag {
	return flagForward
}

// handle implements event
func (t TCPInfo) handle(node *node) {
	node.parent.Send(t)
}

func (t TCPInfo) String() string {
	return fmt.Sprintf("TCPInfo[Flow:%s Location:%s T:%s SampleTime:%s RTT:%s]",
		t.Flow,
		t.Location,
		t.T,
		t.SampleTime,
		t.RTT,
	)
}

// Stop stops the sampler and waits for it to complete. Add must have been
// called successfully at least once first, or this method will hang.
func (s *sockdiagSampler) Stop() {
	close(s.cxl)
	<-s.done
}

// sockAddr contains the identifying addresses for a socket (source and
// destination IP and port), used to find the socket statistics for a flow.
type sockAddr struct {
	SrcIP   net.IP
	SrcPort int
	DstIP   net.IP
	DstPort int
}

// To4 returns sockAddr as a sockAddr4. The return parameter ok is false if this
// is not an IPv4 address.
func (a sockAddr) To4() (addr sockAddr4, ok bool) {
	if len(a.SrcIP) != 4 {
		return
	}
	copy(addr.SrcIP[:], a.SrcIP)
	addr.SrcPort = a.SrcPort
	copy(addr.DstIP[:], a.DstIP)
	addr.DstPort = a.DstPort
	ok = true
	return
}

// To4 returns sockAddr as a sockAddr6. The return parameter ok is false if this
// is not an IPv6 address.
func (a sockAddr) To6() (addr sockAddr6, ok bool) {
	if len(a.SrcIP) != 16 {
		return
	}
	copy(addr.SrcIP[:], a.SrcIP)
	addr.SrcPort = a.SrcPort
	copy(addr.DstIP[:], a.DstIP)
	addr.DstPort = a.DstPort
	ok = true
	return
}

func (a sockAddr) String() string {
	return fmt.Sprintf("sockAddr[%s:%d %s:%d]",
		a.SrcIP, a.SrcPort, a.DstIP, a.DstPort)
}

// sockAddr4 contains an IPv4 socket address.
type sockAddr4 struct {
	SrcIP   [4]byte
	SrcPort int
	DstIP   [4]byte
	DstPort int
}

// sampleSockAddr4 returns a sockAddr4 for the given sample from C.
func sampleSockAddr4(s C.struct_sample) (addr sockAddr4) {
	for i := 0; i < 4; i++ {
		addr.SrcIP[i] = byte(s.saddr[i])
	}
	addr.SrcPort = int(s.sport)
	for i := 0; i < 4; i++ {
		addr.DstIP[i] = byte(s.daddr[i])
	}
	addr.DstPort = int(s.dport)
	return
}

func (a sockAddr4) String() string {
	return fmt.Sprintf("sockAddr4[%s:%d %s:%d]",
		net.IP(a.SrcIP[:]), a.SrcPort, net.IP(a.DstIP[:]), a.DstPort)
}

// sockAddr6 contains an IPv6 socket address.
type sockAddr6 struct {
	SrcIP   [16]byte
	SrcPort int
	DstIP   [16]byte
	DstPort int
}

// sampleSockAddr6 returns a sockAddr6 for the given sample from C.
func sampleSockAddr6(s C.struct_sample) (addr sockAddr6) {
	for i := 0; i < 16; i++ {
		addr.SrcIP[i] = byte(s.saddr[i])
	}
	addr.SrcPort = int(s.sport)
	for i := 0; i < 16; i++ {
		addr.DstIP[i] = byte(s.daddr[i])
	}
	addr.DstPort = int(s.dport)
	return
}

func (a sockAddr6) String() string {
	return fmt.Sprintf("sockAddr6[%s:%d %s:%d]",
		net.IP(a.SrcIP[:]), a.SrcPort, net.IP(a.DstIP[:]), a.DstPort)
}
