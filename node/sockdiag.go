// SPDX-License-Identifier: GPL-3.0
// Copyright 2024 Pete Heist

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
	"net/netip"
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
	sampler map[time.Duration]*sampler
	mtx     sync.Mutex
	cxl     chan struct{}
}

// newSockdiag returns a new sockdiag.
func newSockdiag(ev chan event) *sockdiag {
	return &sockdiag{
		ev,
		make(map[time.Duration]*sampler),
		sync.Mutex{},
		make(chan struct{}),
	}
}

// Add adds the given socket address for TCPInfo sampling at the given interval.
// Since Flow corresponds to the 5-tuple for TCP, the Flow in the given id
// must uniquely identify the src and dst socket addresses in addr.
func (d *sockdiag) Add(addr sockAddr, id TCPInfoID, interval time.Duration) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var s *sampler
	if s = d.sampler[interval]; s == nil {
		s = newSampler(d.ev, interval)
		d.sampler[interval] = s
	}
	s.Add(addr, id)
}

// Remove stops sampling for the given sock address, at the given interval.
func (d *sockdiag) Remove(addr sockAddr, interval time.Duration) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var s *sampler
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

// sampler samples socket statistics on a fixed interval, and sends
// TCPInfo's with the statistics to the node's event channel.
type sampler struct {
	addr     map[sockAddr]TCPInfoID
	addr4    int
	addr6    int
	ev       chan event
	interval time.Duration
	mtx      sync.Mutex
	started  bool
	cxl      chan struct{}
	done     chan struct{}
}

// newSampler returns a new sampler that samples socket statistics on the given
// interval.
func newSampler(ev chan event, interval time.Duration) *sampler {
	return &sampler{
		make(map[sockAddr]TCPInfoID),
		0,
		0,
		ev,
		interval,
		sync.Mutex{},
		false,
		make(chan struct{}),
		make(chan struct{}),
	}
}

// Add registers the given socket address to send TCPInfo for, with the given
// flow id. If this is the first address added, the sampling goroutine is
// started.
func (m *sampler) Add(addr sockAddr, id TCPInfoID) {
	m.mtx.Lock()
	defer func() {
		if !m.started && len(m.addr) > 0 {
			m.started = true
			go m.run()
		}
		m.mtx.Unlock()
	}()
	if _, ok := m.addr[addr]; !ok {
		if addr.Is4() {
			m.addr4++
		} else {
			m.addr6++
		}
	}
	m.addr[addr] = id
}

// TCPFlowInfo contains the flow and orientation information in TCPInfo.
type TCPInfoID struct {
	Flow     Flow
	Location Location
}

// Remove unregisters the given socket address for sampling.
func (m *sampler) Remove(addr sockAddr) (empty bool) {
	m.mtx.Lock()
	defer func() {
		empty = len(m.addr) == 0
		m.mtx.Unlock()
	}()
	if _, ok := m.addr[addr]; ok {
		delete(m.addr, addr)
		if addr.Is4() {
			m.addr4++
		} else {
			m.addr6++
		}
	}
	return
}

// run is the entry point for the sampler goroutine.
func (m *sampler) run() {
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
func (m *sampler) sample(fd C.int) (err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if m.addr4 > 0 {
		if err = m.sampleFamily(fd, unix.AF_INET); err != nil {
			return
		}
	}
	if m.addr6 > 0 {
		err = m.sampleFamily(fd, unix.AF_INET6)
	}
	return
}

// sampleFamily uses netlink to get tcp_info arrays for the given socket family
// (AF_INET or AF_INET6), and sends TCPInfo's for each address registered with
// the sampler.
func (m *sampler) sampleFamily(fd C.int, family C.uchar) (err error) {
	var cs C.struct_samples
	t0 := metric.Now()
	if _, err = C.sockdiag_sample(fd, family, &cs); err != nil {
		return
	}
	t := metric.Now()
	ss := (*[1 << 30]C.struct_sample)(unsafe.Pointer(cs.sample))[:cs.len:cs.len]
	for _, s := range ss {
		var ok bool
		var id TCPInfoID
		if id, ok = m.addr[sockAddrSample(s)]; !ok {
			continue
		}
		m.ev <- newTCPInfo(id, t, time.Duration(t-t0), s.info)
	}
	C.sockdiag_free_samples(&cs)
	return
}

// TCPInfo contains a subset of the socket statistics from Linux's tcp_info
// struct, defined in include/uapi/linux/tcp.h.
type TCPInfo struct {
	TCPInfoID

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
func newTCPInfo(id TCPInfoID, t metric.RelativeTime, st time.Duration,
	ti C.struct_tcp_info) TCPInfo {
	return TCPInfo{
		id,
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
	return fmt.Sprintf("TCPInfo[Flow:%s Location:%s T:%s SampleTime:%s "+
		"RTT:%s RTTVar:%s TotalRetransmits:%d DeliveryRate:%s PacingRate: %s "+
		"SendCwnd:%d SendMSS:%s]",
		t.Flow,
		t.Location,
		t.T,
		t.SampleTime,
		t.RTT,
		t.RTTVar,
		t.TotalRetransmits,
		t.DeliveryRate,
		t.PacingRate,
		t.SendCwnd,
		t.SendMSS,
	)
}

// Stop stops the sampler and waits for it to complete. Add must have been
// called successfully at least once first, or this method will hang.
func (s *sampler) Stop() {
	close(s.cxl)
	<-s.done
}

// sockAddr contains the identifying addresses for a socket (source and
// destination IP and port), used to find the socket statistics for a flow.
type sockAddr struct {
	Src netip.AddrPort
	Dst netip.AddrPort
}

// sockAddrConn returns a sockAddr for the given Conn.
func sockAddrConn(c net.Conn) (addr sockAddr) {
	addr.Src = c.LocalAddr().(*net.TCPAddr).AddrPort()
	addr.Dst = c.RemoteAddr().(*net.TCPAddr).AddrPort()
	return
}

// sockAddrSample returns a sockAddr for the given sample from C.
func sockAddrSample(s C.struct_sample) (addr sockAddr) {
	var sa, da netip.Addr
	switch s.family {
	case unix.AF_INET:
		var b [4]byte
		for i := 0; i < 4; i++ {
			b[i] = byte(s.saddr[i])
		}
		sa = netip.AddrFrom4(b)
		for i := 0; i < 4; i++ {
			b[i] = byte(s.daddr[i])
		}
		da = netip.AddrFrom4(b)
	case unix.AF_INET6:
		var b [16]byte
		for i := 0; i < 16; i++ {
			b[i] = byte(s.saddr[i])
		}
		sa = netip.AddrFrom16(b)
		for i := 0; i < 16; i++ {
			b[i] = byte(s.daddr[i])
		}
		da = netip.AddrFrom16(b)
	}
	addr.Src = netip.AddrPortFrom(sa, uint16(s.sport))
	addr.Dst = netip.AddrPortFrom(da, uint16(s.dport))
	return
}

// Is4 returns true if this is an IPv4 sockAddr.
func (a sockAddr) Is4() bool {
	return a.Src.Addr().Is4()
}

func (a sockAddr) String() string {
	return fmt.Sprintf("sockAddr[Src:%s Dst:%s]", a.Src, a.Dst)
}
