// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package antler

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/heistp/antler/node"
	"github.com/heistp/antler/node/metric"
	"gonum.org/v1/gonum/stat"
)

// LinuxSSThreshInfinity is the initial value of ssthresh in Linux.
const LinuxSSThreshInfinity = 2147483647

// Analyze is a reporter that processes stream and packet data for reports.
// This must be in the Report pipeline *before* reporters that require it.
type Analyze struct {
}

// report implements reporter
func (Analyze) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	y := newAnalysis()
	for d := range in {
		out <- d
		y.add(d)
	}
	y.analyze()
	out <- y
	return
}

// analysis contains the results of the Analyze reporter.
type analysis struct {
	streams streams
	packets packets
}

// newAnalysis returns a new analysis.
func newAnalysis() analysis {
	return analysis{
		newStreams(),
		newPackets(),
	}
}

// add adds a data item from the result stream.
func (y *analysis) add(a any) {
	switch v := a.(type) {
	case node.StreamInfo:
		s := y.streams.analysis(v.Flow)
		if v.Server {
			s.Server = v
		} else {
			s.Client = v
		}
	case node.StreamIO:
		s := y.streams.analysis(v.Flow)
		if v.Sent {
			s.Sent = append(s.Sent, v)
		} else {
			s.Rcvd = append(s.Rcvd, v)
		}
	case node.TCPInfo:
		s := y.streams.analysis(v.Flow)
		s.TCPInfo = append(s.TCPInfo, v)
	case node.PacketInfo:
		p := y.packets.analysis(v.Flow)
		if v.Server {
			p.Server = v
		} else {
			p.Client = v
		}
	case node.PacketIO:
		p := y.packets.analysis(v.Flow)
		if v.Server {
			if v.Sent {
				p.ServerSent = append(p.ServerSent, v)
			} else {
				p.ServerRcvd = append(p.ServerRcvd, v)
			}
		} else {
			if v.Sent {
				p.ClientSent = append(p.ClientSent, v)
			} else {
				p.ClientRcvd = append(p.ClientRcvd, v)
			}
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (y *analysis) analyze() {
	ss := y.streams.StartTime()
	ps := y.packets.StartTime()
	st := ss
	if st.IsZero() || (!ps.IsZero() && ps.Before(st)) {
		st = ps
	}
	y.streams.synchronize(st)
	y.packets.synchronize(st)
	y.streams.analyze()
	y.packets.analyze()
}

// StreamAnalysis contains the data and calculated stats for a stream.
type StreamAnalysis struct {
	Flow         node.Flow
	Client       node.StreamInfo
	Server       node.StreamInfo
	Sent         []node.StreamIO
	Rcvd         []node.StreamIO
	TCPInfo      []node.TCPInfo
	GoodputPoint []GoodputPoint
	RtxCumAvg    []rtxCumAvg
	FCT          metric.Duration
	Length       metric.Bytes
	SSExitTime   metric.RelativeTime
}

// T0 returns the earliest absolute time from Sent or Rcvd.
func (s *StreamAnalysis) T0() time.Time {
	if len(s.Sent) == 0 {
		if len(s.Rcvd) == 0 {
			return time.Time{}
		}
		return s.Server.Time(s.Rcvd[0].T)
	} else if len(s.Rcvd) == 0 {
		return s.Client.Time(s.Sent[0].T)
	} else {
		if s.Sent[0].T < s.Rcvd[0].T {
			return s.Client.Time(s.Sent[0].T)
		} else {
			return s.Server.Time(s.Rcvd[0].T)
		}
	}
}

// Goodput returns the total goodput for the stream.
func (s *StreamAnalysis) Goodput() metric.Bitrate {
	return metric.CalcBitrate(s.Length, s.FCT.Duration())
}

// GoodputPoint is a single Goodput data point.
type GoodputPoint struct {
	// T is the time relative to the start of the earliest stream.
	T metric.RelativeTime

	// Goodput is the goodput bitrate.
	Goodput metric.Bitrate
}

// rtxCumAvg is a single cumulative average retransmission data point.
type rtxCumAvg struct {
	// T is the time relative to the start of the earliest stream.
	T metric.RelativeTime

	// RtxCumAvg is the cumulative average retransmission rate, in
	// retransmissions / sec.
	RtxCumAvg float64
}

// streams aggregates data for multiple streams.
type streams map[node.Flow]*StreamAnalysis

// newStreams returns a new streams.
func newStreams() streams {
	return streams(make(map[node.Flow]*StreamAnalysis))
}

// analysis adds streamAnalysis for the given flow if it doesn't already exist.
func (m *streams) analysis(flow node.Flow) (s *StreamAnalysis) {
	var ok bool
	if s, ok = (*m)[flow]; ok {
		return
	}
	s = &StreamAnalysis{Flow: flow}
	(*m)[flow] = s
	return
}

// StartTime returns the earliest absolute start time among the streams.
func (m *streams) StartTime() (start time.Time) {
	for _, s := range *m {
		t0 := s.T0()
		if start.IsZero() || t0.Before(start) {
			start = t0
		}
	}
	return
}

// synchronize adjusts the StreamIO RelativeTime values from node-relative to
// test-relative time.
func (m *streams) synchronize(start time.Time) {
	for _, r := range *m {
		for i := 0; i < len(r.Sent); i++ {
			io := &r.Sent[i]
			t := io.T.Time(r.Client.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(r.Rcvd); i++ {
			io := &r.Rcvd[i]
			t := io.T.Time(r.Server.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(r.TCPInfo); i++ {
			n := &r.TCPInfo[i]
			t := n.T.Time(r.Server.Tinit)
			n.T = metric.RelativeTime(t.Sub(start))
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (m *streams) analyze() {
	for _, s := range *m {
		var pr node.StreamIO
		for i := 0; i < len(s.Rcvd)-1; i++ {
			r := s.Rcvd[i]
			var g metric.Bitrate
			if pr != (node.StreamIO{}) {
				g = metric.CalcBitrate(r.Total-pr.Total,
					time.Duration(r.T-pr.T))
			}
			s.GoodputPoint = append(s.GoodputPoint, GoodputPoint{r.T, g})
			pr = r
		}
		var sx bool
		for i := 0; i < len(s.TCPInfo); i++ {
			t := s.TCPInfo[i]
			r := float64(t.TotalRetransmits) / t.T.Duration().Seconds()
			s.RtxCumAvg = append(s.RtxCumAvg, rtxCumAvg{t.T, r})
			if !sx && t.SendSSThresh < LinuxSSThreshInfinity {
				s.SSExitTime = t.T
				sx = true
			}
		}
		if !sx {
			s.SSExitTime = metric.RelativeTime(-1)
		}
		if len(s.Rcvd) > 0 {
			s.Length = s.Rcvd[len(s.Rcvd)-1].Total
			if len(s.Sent) > 0 {
				s.FCT = metric.Duration(s.Rcvd[len(s.Rcvd)-1].T - s.Sent[0].T)
			}
		}
	}
}

// byTime returns a slice of streamAnalysis, sorted by start time.
func (m *streams) byTime() (s []StreamAnalysis) {
	for _, d := range *m {
		s = append(s, *d)
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].T0().Before(s[j].T0())
	})
	return
}

// PacketAnalysis contains the data and calculated stats for a packet flow.
type PacketAnalysis struct {
	// data
	Flow       node.Flow
	Client     node.PacketInfo
	Server     node.PacketInfo
	ClientSent []node.PacketIO
	ClientRcvd []node.PacketIO
	ServerSent []node.PacketIO
	ServerRcvd []node.PacketIO

	// statistics
	Up      packetStats // stats from client to server
	Down    packetStats // stats from server to client
	RTT     []rtt
	RTTMean float64
}

// packetStats contains statistics for one direction of a packet flow.
type packetStats struct {
	Lost     []lost
	LostPct  float64
	Dup      []dup
	DupPct   float64
	OWD      []owd
	OWDMean  float64
	Early    []early
	EarlyPct float64
	Late     []late
	LatePct  float64
}

// owd is a single one-way delay data point.
type owd struct {
	T     metric.RelativeTime // time the packet was received
	Seq   node.Seq            // sequence number of sample
	Delay time.Duration       // one-way delay
}

// rtt is a single round-trip time data point.
type rtt struct {
	T     metric.RelativeTime // time the packet was received
	Seq   node.Seq            // round-trip sequence number
	Delay time.Duration       // round-trip time
}

// lost is a single lost packet data point.
type lost struct {
	T   metric.RelativeTime // time the packet was sent
	Seq node.Seq            // sequence number that was lost
}

// late is a single late packet data point.
type late struct {
	T   metric.RelativeTime // time the packet was received
	Seq node.Seq            // sequence number that was late
}

// early is a single early packet data point.
type early struct {
	T   metric.RelativeTime // time the packet was received
	Seq node.Seq            // sequence number that was early
}

// dup is a single duplicate packet data point.
type dup struct {
	T   metric.RelativeTime // time the packet was received
	Seq node.Seq            // sequence number of duplicate
}

// analyze records the one-way packet stats from source and dest packets. The
// destination map is returned for optional further analysis.
func (s *packetStats) analyze(src, dst []node.PacketIO) (
	dstMap map[node.Seq]node.PacketIO) {
	srcLen := len(src)
	// create dst map, find dups and remove from dst
	dstMap = make(map[node.Seq]node.PacketIO)
	var dst2 []node.PacketIO
	for _, dp := range dst {
		if _, ok := dstMap[dp.Seq]; ok {
			//fmt.Printf("dup %d\n", dp.Seq)
			s.Dup = append(s.Dup, dup{dp.T, dp.Seq})
			continue
		}
		dstMap[dp.Seq] = dp
		dst2 = append(dst2, dp)
	}
	dst = dst2
	// find lost packets and remove from src, and record OWD along the way
	var src2 []node.PacketIO
	for _, sp := range src {
		dp, ok := dstMap[sp.Seq]
		if !ok {
			//fmt.Printf("lost %d\n", sp.Seq)
			s.Lost = append(s.Lost, lost{sp.T, sp.Seq})
			continue
		}
		s.OWD = append(s.OWD, owd{dp.T, sp.Seq, time.Duration(dp.T - sp.T)})
		src2 = append(src2, sp)
	}
	src = src2
	if len(src) != len(dst) {
		panic(fmt.Sprintf("packetStats.analyze len(src)=%d != len(dst)=%d (incoherent data, re-run test)",
			len(src), len(dst)))
	}
	// find early and late packets
	for i := 0; i < len(src); i++ {
		sp := src[i]
		dp := dst[i]
		if dp.Seq < sp.Seq {
			//fmt.Printf("late %d\n", dp.Seq)
			s.Late = append(s.Late, late{dp.T, dp.Seq})
		} else if dp.Seq > sp.Seq {
			//fmt.Printf("early %d\n", dp.Seq)
			s.Early = append(s.Early, early{dp.T, dp.Seq})
		}
	}
	// summary stats
	var oo []float64
	for _, o := range s.OWD {
		oo = append(oo, o.Delay.Seconds()*1000.0)
	}
	s.OWDMean = stat.Mean(oo, nil)
	s.LostPct = 100.0 * float64(len(s.Lost)) / float64(srcLen)
	s.DupPct = 100.0 * float64(len(s.Dup)) / float64(srcLen)
	s.EarlyPct = 100.0 * float64(len(s.Early)) / float64(srcLen)
	s.LatePct = 100.0 * float64(len(s.Late)) / float64(srcLen)
	return
}

// T0 returns the earliest absolute packet time.
func (y *PacketAnalysis) T0() time.Time {
	if len(y.ClientSent) == 0 {
		if len(y.ServerRcvd) == 0 {
			return time.Time{}
		}
		return y.Server.Time(y.ServerRcvd[0].T)
	} else if len(y.ServerRcvd) == 0 {
		return y.Client.Time(y.ClientSent[0].T)
	} else {
		if y.ClientSent[0].T < y.ServerRcvd[0].T {
			return y.Client.Time(y.ClientSent[0].T)
		} else {
			return y.Server.Time(y.ServerRcvd[0].T)
		}
	}
}

// analyze gets the packet statistics for the Flow. The data fields must already
// have been populated.
func (y *PacketAnalysis) analyze() {
	//fmt.Printf("analyze ClientSent:%d ServerRcvd:%d\n",
	//	len(y.ClientSent), len(y.ServerRcvd))
	// analyze stats for each direction
	y.Up.analyze(y.ClientSent, y.ServerRcvd)
	//fmt.Printf("analyze ServerSent:%d ClientRcvd:%d\n",
	//	len(y.ServerSent), len(y.ClientRcvd))
	d := y.Down.analyze(y.ServerSent, y.ClientRcvd)
	// get round-trip times
	var rr []float64
	for _, sp := range y.ClientSent {
		if dp, ok := d[sp.Seq]; ok {
			r := time.Duration(dp.T - sp.T)
			y.RTT = append(y.RTT, rtt{dp.T, sp.Seq, r})
			rr = append(rr, r.Seconds()*1000.0)
			//fmt.Printf("rtt %d\n", r)
		}
	}
	y.RTTMean = stat.Mean(rr, nil)
}

// packets aggregates data for multiple packet flows.
type packets map[node.Flow]*PacketAnalysis

// newPackets returns a new packets.
func newPackets() packets {
	return packets(make(map[node.Flow]*PacketAnalysis))
}

// analysis adds packetAnalysis for the given flow if it doesn't already exist.
func (k *packets) analysis(flow node.Flow) (d *PacketAnalysis) {
	var ok bool
	if d, ok = (*k)[flow]; ok {
		return
	}
	d = &PacketAnalysis{Flow: flow}
	(*k)[flow] = d
	return
}

// StartTime returns the earliest absolute start time among the packet flows.
func (k *packets) StartTime() (start time.Time) {
	for _, d := range *k {
		t0 := d.T0()
		if start.IsZero() || t0.Before(start) {
			start = t0
		}
	}
	return
}

// synchronize adjusts the PacketIO RelativeTime values from node-relative to
// test-relative time.
func (k *packets) synchronize(start time.Time) {
	for _, p := range *k {
		for i := 0; i < len(p.ClientSent); i++ {
			io := &p.ClientSent[i]
			t := io.T.Time(p.Client.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(p.ServerRcvd); i++ {
			io := &p.ServerRcvd[i]
			t := io.T.Time(p.Server.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(p.ServerSent); i++ {
			io := &p.ServerSent[i]
			t := io.T.Time(p.Server.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(p.ClientRcvd); i++ {
			io := &p.ClientRcvd[i]
			t := io.T.Time(p.Client.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (k *packets) analyze() {
	for _, p := range *k {
		p.analyze()
	}
}

// byTime returns a slice of packetAnalysis, sorted by start time.
func (k *packets) byTime() (d []PacketAnalysis) {
	for _, p := range *k {
		d = append(d, *p)
	}
	sort.Slice(d, func(i, j int) bool {
		return d[i].T0().Before(d[j].T0())
	})
	return
}
