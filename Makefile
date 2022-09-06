# disable cgo
env = CGO_ENABLED=0

# race detection
#INSTALL_FLAGS := -race
#env = CGO_ENABLED=1

all: install

.PHONY: install
install: antler-node
	go install $(INSTALL_FLAGS) ./cmd/antler

.PHONY: antler-node
antler-node:
	./Makenode

.PHONY: clean
clean:
	rm -fr node/bin
