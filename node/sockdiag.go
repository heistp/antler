// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

/*
#cgo CFLAGS: -O2 -Wall

//#include <unistd.h>
#include "sockdiag.h"
*/
import "C"

import (
	"fmt"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func sampleSockdiag(fd C.int, family C.uchar) (err error) {
	var ss C.struct_samples
	t0 := time.Now()
	if _, err = C.sockdiag_sample(fd, family, &ss); err != nil {
		return
	}
	el := time.Since(t0)
	fmt.Printf("family: %d samples: %d elapsed: %s\n", family, ss.len, el)
	s := (*[1 << 30]C.struct_sample)(unsafe.Pointer(ss.sample))[:ss.len:ss.len]
	for i := 0; i < int(ss.len); i++ {
		var l int
		if s[i].family == unix.AF_INET {
			l = 4
		} else {
			l = 16
		}
		var sa net.IP = make([]byte, l)
		for j := 0; j < l; j++ {
			sa[j] = byte(s[i].saddr[j])
		}
		var da net.IP = make([]byte, l)
		for j := 0; j < l; j++ {
			da[j] = byte(s[i].daddr[j])
		}
		fmt.Printf("%s:%d %s:%d\n", sa, s[i].sport, da, s[i].dport)
		fmt.Printf("  rtt: %d\n", s[i].info.tcpi_rtt)
		fmt.Printf("  rttvar: %d\n", s[i].info.tcpi_rttvar)
		fmt.Printf("  total_retrans: %d\n", s[i].info.tcpi_total_retrans)
		fmt.Printf("  delivery_rate: %d\n", s[i].info.tcpi_delivery_rate)
		fmt.Printf("  pacing_rate: %d\n", s[i].info.tcpi_pacing_rate)
		fmt.Printf("  snd_cwnd: %d\n", s[i].info.tcpi_snd_cwnd)
		fmt.Printf("  snd_mss: %d\n", s[i].info.tcpi_snd_mss)

		// RTTs in usec
		// snd_cwnd and snd_mss in bytes
		// pacing_rate and delivery_rate in bytes/sec
	}
	C.sockdiag_free_samples(&ss)
	return
}

func TestSockdiag(family C.uchar) (err error) {
	var fd C.int
	if fd, err = C.sockdiag_open(); fd < 0 {
		return
	}
	defer C.sockdiag_close(fd)
	err = sampleSockdiag(fd, family)
	return
}
