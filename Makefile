APP := shiroyagi
BUILD_INFO := $(APP)-build-info.txt
SHIROYAGI_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
SHIROYAGI_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w
LDFLAGS += -X github.com/takayoshiotake/shiroyagi/internal/version.Version=$(SHIROYAGI_VERSION)
LDFLAGS += -X github.com/takayoshiotake/shiroyagi/internal/version.Commit=$(SHIROYAGI_COMMIT)

.PHONY: build

build:
	go build -trimpath -buildvcs=true -ldflags="$(LDFLAGS)" -o $(APP) ./cmd/shiroyagi
	go version -m $(APP) > $(BUILD_INFO)
