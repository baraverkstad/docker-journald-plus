PLUGIN_NAME ?= baraverkstad/journald-plus
PLUGIN_TAG ?= latest
PLUGIN_DIR = tmp/plugin

.PHONY: all clean build build-docker test publish

all:
	@echo '🌈 Makefile commands'
	@grep -E -A 1 '^#' Makefile | awk 'BEGIN { RS = "--\n"; FS = "\n" }; { sub("#+ +", "", $$1); sub(":.*", "", $$2); printf " · make %-18s- %s\n", $$2, $$1}'
	@echo
	@echo '🚀 Release builds'
	@echo ' · make PLUGIN_TAG=v1.0.0 publish'

# Cleanup build artifacts
clean:
	rm -rf tmp
	docker rm -f journald-plus-tmp 2>/dev/null || true
	docker rmi journald-plus-build 2>/dev/null || true

# Compile Go binary
build:
	mkdir -p tmp/build
	CGO_ENABLED=0 go build -ldflags='-s -w' -o tmp/build/journald-plus .

build-docker: build test
	docker build -t journald-plus-build -f Dockerfile .
	mkdir -p $(PLUGIN_DIR)/rootfs
	docker create --name journald-plus-tmp journald-plus-build true
	docker export journald-plus-tmp | tar -x -C $(PLUGIN_DIR)/rootfs
	docker rm journald-plus-tmp
	cp config.json $(PLUGIN_DIR)/

# Run tests & code style checks
test:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Formatting issues in: $$(gofmt -l . | xargs)"; exit 1)
	go test ./...

# Build plugin and push to Docker Hub
publish: PLUGIN_FULL = $(PLUGIN_NAME):$(PLUGIN_TAG)
publish: clean build-docker
	docker plugin create $(PLUGIN_FULL) $(PLUGIN_DIR)
	docker plugin push $(PLUGIN_FULL)
	docker plugin rm $(PLUGIN_FULL)
	@echo "✅ Plugin $(PLUGIN_FULL) published to Docker Hub"
