// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2024 Pete Heist

#ifndef _SOCKDIAG_H_
#define _SOCKDIAG_H_

#include <stdint.h>
#include <linux/tcp.h>

// sample contains the data in one sample returned by sockdiag_sample.
struct sample {
	uint8_t family;         // address family (AF_INET or AF_INET6)
	uint8_t saddr[16];      // source (local) IP address
	uint16_t sport;         // source (local) port
	uint8_t daddr[16];      // dest (remote) IP address
	uint16_t dport;         // dest (remote) port
	struct tcp_info info;   // TCP info
};

// samples is a list of sample's, with length and capacity.
struct samples {
	struct sample *sample;
	uint32_t len;
	uint32_t cap;
};

int sockdiag_open();
int sockdiag_sample(int fd, uint8_t family, struct samples *samples);
void sockdiag_free_samples(struct samples *samples);
int sockdiag_close(int fd);

#endif // _SOCKDIAG_H
