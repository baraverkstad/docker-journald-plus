PLUGIN_NAME ?= baraverkstad/journald-plus
PLUGIN_TAG ?= latest
PLUGIN_FULL = $(PLUGIN_NAME):$(PLUGIN_TAG)
PLUGIN_DIR = plugin-dir

.PHONY: build test publish plugin rootfs enable disable push clean

all:
	@echo '🌈 Makefile commands'
	@grep -E -A 1 '^#' Makefile | awk 'BEGIN { RS = "--\n"; FS = "\n" }; { sub("#+ +", "", $$1); sub(":.*", "", $$2); printf " · make %-18s- %s\n", $$2, $$1}'

# Compile Go binary (local development)
build:
	CGO_ENABLED=0 go build -ldflags='-s -w' -o journald-plus .

# Run tests & code style checks
test:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Formatting issues in: $$(gofmt -l . | xargs)"; exit 1)
	go test ./...

# Build plugin and push to Docker Hub
publish: plugin push
	@echo "✅ Plugin $(PLUGIN_FULL) published to Docker Hub"

# Create Docker plugin (for testing/development)
plugin: rootfs
	docker plugin create $(PLUGIN_FULL) $(PLUGIN_DIR)

# Build plugin rootfs (internal build step)
rootfs: clean-rootfs
	docker build -t journald-plus-build -f Dockerfile .
	mkdir -p $(PLUGIN_DIR)/rootfs
	docker create --name journald-plus-tmp journald-plus-build true
	docker export journald-plus-tmp | tar -x -C $(PLUGIN_DIR)/rootfs
	docker rm journald-plus-tmp
	cp config.json $(PLUGIN_DIR)/

# Enable plugin locally (for testing)
enable:
	docker plugin enable $(PLUGIN_FULL)

# Disable plugin locally (for testing)
disable:
	docker plugin disable $(PLUGIN_FULL)

# Push plugin to Docker Hub (use 'make publish' instead)
push:
	docker plugin push $(PLUGIN_FULL)

clean: clean-rootfs
	rm -f journald-plus

clean-rootfs:
	rm -rf $(PLUGIN_DIR)
	-docker rm -f journald-plus-tmp 2>/dev/null
