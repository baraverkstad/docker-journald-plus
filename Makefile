COMMIT  := $(shell GIT_CONFIG_GLOBAL=/dev/null git rev-parse --short=8 HEAD)
DATE    := $(or $(DATE),$(shell date '+%F'))
VER     := $(if $(VERSION),$(patsubst v%,%,$(VERSION)),$(shell date '+%Y.%m.%d').$(COMMIT)-SNAPSHOT)
REPO    := baraverkstad/journald-plus
TAG     := $(or $(VERSION),latest)

.PHONY: all clean build test publish

all:
	@echo '🌈 Makefile commands'
	@grep -E -A 1 '^#' Makefile | awk 'BEGIN { RS = "--\n"; FS = "\n" }; { sub("#+ +", "", $$1); sub(":.*", "", $$2); printf " · make %-18s- %s\n", $$2, $$1}'
	@echo
	@echo '🚀 Release builds'
	@echo ' · make VERSION=v1.0.0 clean build test publish'

# Cleanup build artifacts
clean:
	rm -rf tmp

# Compile Go binary
build:
	mkdir -p tmp/build
	CGO_ENABLED=0 go build -ldflags='-s -w' -o tmp/build/journald-plus .

# Run tests & code style checks
test:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "Formatting issues in: $$(gofmt -l . | xargs)"; exit 1)
	go test ./...

define publish-arch
	@echo "📦 Building linux/$(1)..."
	mkdir -p tmp/plugin-$(1)/rootfs
	docker buildx build --platform linux/$(1) \
		--output type=local,dest=tmp/plugin-$(1)/rootfs .
	cp config.json tmp/plugin-$(1)/
	docker plugin create $(REPO):$(TAG)-$(1) tmp/plugin-$(1)
	docker plugin push $(REPO):$(TAG)-$(1)
	docker plugin rm $(REPO):$(TAG)-$(1)
endef

# Build and push plugin for multiple architectures
publish: clean
	$(call publish-arch,amd64)
	$(call publish-arch,arm64)
	docker plugin create $(REPO):$(TAG) tmp/plugin-amd64
	docker plugin push $(REPO):$(TAG)
	docker plugin rm $(REPO):$(TAG)
	@echo "✅ Published $(REPO):{$(TAG),$(TAG)-amd64,$(TAG)-arm64}"
