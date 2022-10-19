// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <linux/netlink.h>
#include <linux/rtnetlink.h>
#include <linux/sock_diag.h>
#include <linux/inet_diag.h>

#include "sockdiag.h"

// Kernel tcp states (from net/tcp_states.h).
enum {
	TCP_ESTABLISHED = 1,
};

// sockdiag_open opens a netlink socket and sets sockopts.
int sockdiag_open() {
	// create socket
	int fd;
	if ((fd = socket(AF_NETLINK, SOCK_DGRAM, NETLINK_SOCK_DIAG)) == -1)
		goto socket_failure;

	// set timeout to 1s
	struct timeval t = {0};
	t.tv_sec = 1;
	if (setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &t, sizeof t) == -1)
		goto sockopt_failure;

	// return file descriptor
	return fd;

	// handle errors
sockopt_failure:
	close(fd);
socket_failure:
	return -1;
}

// send_request sends one inet_diag request and returns the result from sendmsg.
int send_request(int fd, uint8_t family) {
	struct sockaddr_nl a = {0};
	a.nl_family = AF_NETLINK;

	struct inet_diag_req_v2 r = {0};
	r.sdiag_family = family;
	r.sdiag_protocol = IPPROTO_TCP;
	r.idiag_states = (1 << TCP_ESTABLISHED);
	r.idiag_ext |= (1 << (INET_DIAG_INFO - 1));

	struct nlmsghdr h = {0};
	h.nlmsg_len = NLMSG_LENGTH(sizeof(r));
	h.nlmsg_flags = NLM_F_DUMP | NLM_F_REQUEST;
	h.nlmsg_type = SOCK_DIAG_BY_FAMILY;

	struct iovec v[2];
	v[0].iov_base = (void*) &h;
	v[0].iov_len = sizeof(h);
	v[1].iov_base = (void*) &r;
	v[1].iov_len = sizeof(r);

	struct msghdr m = {0};
	m.msg_name = (void*) &a;
	m.msg_namelen = sizeof(a);
	m.msg_iov = v;
	m.msg_iovlen = 2;

	return sendmsg(fd, &m, 0);
}

// grow increases the size of the samples array.
#define INIT_CAP 16
void grow(struct samples *samples) {
	if (samples->cap == 0) {
		samples->cap = INIT_CAP;
	} else {
		samples->cap *= 2;
	}
	samples->sample = realloc(samples->sample,
		samples->cap * sizeof(struct sample));
}

// parse_response reads a message and appends samples for each embedded tcp_info.
void parse_response(struct inet_diag_msg *msg, int rtalen,
		struct samples *samples) {
	struct rtattr *attr = (struct rtattr*) (msg+1);
	while (RTA_OK(attr, rtalen)) {
		if(attr->rta_type == INET_DIAG_INFO){
			struct tcp_info *t = (struct tcp_info*) RTA_DATA(attr);
			if (samples->len >= samples->cap) {
				grow(samples);
			}
			samples->sample[samples->len] = (struct sample) {
				msg->idiag_family,
				{0},
				htons(msg->id.idiag_sport),
				{0},
				htons(msg->id.idiag_dport),
				*t,
			};
			int al = msg->idiag_family == AF_INET ? 4 : 16;
			memcpy(samples->sample[samples->len].saddr,
					msg->id.idiag_src, al);
			memcpy(samples->sample[samples->len].daddr,
					msg->id.idiag_dst, al);
			samples->len++;
		}
		attr = RTA_NEXT(attr, rtalen); 
	}
}

// sockdiag_sample sends an inet_diag request, parses the results and returns
// a samples array.
int sockdiag_sample(int fd, uint8_t family, struct samples *samples) {
	// send request
	if (send_request(fd, family) < 0)
		return -1;

	// read until message with NLMSG_DONE is received
	*samples = (const struct samples){0};
	grow(samples);
	while (1) {
		uint8_t b[32*1024];
		int n;
		if ((n = recv(fd, b, sizeof(b), 0)) == -1)
			return -1;

		struct nlmsghdr *h = (struct nlmsghdr*) b;
		while (NLMSG_OK(h, n)) {
			if(h->nlmsg_type == NLMSG_DONE) {
				return 0;
			}

			if(h->nlmsg_type == NLMSG_ERROR) {
				struct nlmsgerr *e = (struct nlmsgerr*)NLMSG_DATA(h);
				if (h->nlmsg_len < NLMSG_LENGTH(sizeof(struct nlmsgerr)))
					errno = ENODATA;
				else
					errno = -e->error;
				return -1;
			}

			struct inet_diag_msg *m =
				(struct inet_diag_msg*) NLMSG_DATA(h);

			int rl = h->nlmsg_len - NLMSG_LENGTH(sizeof(*m));
			if (rl > 0)
				parse_response(m, rl, samples);

			h = NLMSG_NEXT(h, n); 
		}
	}
}

// sockdiag_free_samples deallocates a samples list.
void sockdiag_free_samples(struct samples *samples) {
	free(samples->sample);
}

// sockdiag_close closes a netlink socket.
int sockdiag_close(int fd) {
	return close(fd);
}
