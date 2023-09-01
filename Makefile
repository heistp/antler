# race detection
#INSTALL_FLAGS := -race

all: install

.PHONY: install
install: antler-node
	go install $(INSTALL_FLAGS) ./cmd/antler

.PHONY: antler-node
antler-node:
	./Makenode

.PHONY: deploy
deploy: install
	sudo systemctl stop antler
	cp ~/go/bin/antler /usr/local/bin
	sudo systemctl start antler

.PHONY: clean
clean:
	rm -fr node/bin
