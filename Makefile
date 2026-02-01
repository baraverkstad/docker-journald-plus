PLUGIN_NAME ?= baraverkstad/journald-plus
PLUGIN_TAG ?= latest
PLUGIN_FULL = $(PLUGIN_NAME):$(PLUGIN_TAG)
PLUGIN_DIR = plugin-dir

.PHONY: build rootfs plugin enable disable push clean test

all:
	@echo '🌈 Makefile commands'
	@grep -E -A 1 '^#' Makefile | awk 'BEGIN { RS = "--\n"; FS = "\n" }; { sub("#+ +", "", $$1); sub(":.*", "", $$2); printf " · make %-18s- %s\n", $$2, $$1}'

# Compile sources
build:
	CGO_ENABLED=0 go build -ldflags='-s -w' -o journald-plus .

# Run tests & code style checks
test:
	go test ./...

rootfs: clean-rootfs
	docker build -t journald-plus-build -f Dockerfile .
	mkdir -p $(PLUGIN_DIR)/rootfs
	docker create --name journald-plus-tmp journald-plus-build true
	docker export journald-plus-tmp | tar -x -C $(PLUGIN_DIR)/rootfs
	docker rm journald-plus-tmp
	cp config.json $(PLUGIN_DIR)/

plugin: rootfs
	docker plugin create $(PLUGIN_FULL) $(PLUGIN_DIR)

enable:
	docker plugin enable $(PLUGIN_FULL)

disable:
	docker plugin disable $(PLUGIN_FULL)

push:
	docker plugin push $(PLUGIN_FULL)

clean: clean-rootfs
	rm -f journald-plus

clean-rootfs:
	rm -rf $(PLUGIN_DIR)
	-docker rm -f journald-plus-tmp 2>/dev/null
