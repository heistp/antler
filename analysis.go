// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import (
	"context"
	"sort"
	"time"

	"github.com/heistp/antler/node"
	"github.com/heistp/antler/node/metric"
)

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
	case node.PacketInfo:
		p := y.packets.analysis(v.Flow)
		if v.Server {
			p.Server = v
		} else {
			p.Client = v
		}
	case node.PacketIO:
		p := y.packets.analysis(v.Flow)
		if v.Sent {
			p.Sent = append(p.Sent, v)
		} else {
			p.Rcvd = append(p.Rcvd, v)
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
	GoodputPoint []GoodputPoint
	FCT          metric.Duration
	Length       metric.Bytes
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

// owd is a single one-way delay data point.
type owd struct {
	// T is the time relative to the start of the test.
	T metric.RelativeTime

	// Delay is the one-way delay.
	Delay time.Duration
}

// PacketAnalysis contains the data and calculated stats for a packet flow.
type PacketAnalysis struct {
	Flow   node.Flow
	Client node.PacketInfo
	Server node.PacketInfo
	Sent   []node.PacketIO
	Rcvd   []node.PacketIO
	OWD    []owd
}

// T0 returns the earliest absolute packet time.
func (y *PacketAnalysis) T0() time.Time {
	if len(y.Sent) == 0 {
		if len(y.Rcvd) == 0 {
			return time.Time{}
		}
		return y.Server.Time(y.Rcvd[0].T)
	} else if len(y.Rcvd) == 0 {
		return y.Client.Time(y.Sent[0].T)
	} else {
		if y.Sent[0].T < y.Rcvd[0].T {
			return y.Client.Time(y.Sent[0].T)
		} else {
			return y.Server.Time(y.Rcvd[0].T)
		}
	}
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
		for i := 0; i < len(p.Sent); i++ {
			io := &p.Sent[i]
			t := io.T.Time(p.Client.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
		for i := 0; i < len(p.Rcvd); i++ {
			io := &p.Rcvd[i]
			t := io.T.Time(p.Server.Tinit)
			io.T = metric.RelativeTime(t.Sub(start))
		}
	}
}

// analyze uses the collected data to calculate relevant metrics and stats.
func (k *packets) analyze() {
	for _, p := range *k {
		var s, r node.PacketIO
		for _, s = range p.Sent {
			for _, r = range p.Rcvd {
				if s.Seq == r.Seq {
					d := time.Duration(r.T - s.T)
					p.OWD = append(p.OWD, owd{r.T, d})
				}
			}
		}
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
