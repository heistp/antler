# SPDX-License-Identifier: GPL-3.0
# Copyright 2024 Pete Heist

# race detection
#INSTALL_FLAGS := -race

all: install

.PHONY: install
install: antler-node
	go install $(INSTALL_FLAGS) ./cmd/antler
	@if [ -x /usr/local/sbin/antler-deploy ]; then \
		/usr/local/sbin/antler-deploy; \
	fi

.PHONY: antler-node
antler-node:
	./Makenode

.PHONY: clean
clean:
	rm -fr node/bin
