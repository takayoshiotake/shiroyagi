APP := shiroyagi
BUILD_INFO := $(APP)-build-info.txt
DIST_DIR := dist

SHIROYAGI_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
SHIROYAGI_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

ARCHIVE_NAME := $(APP)_$(SHIROYAGI_VERSION)_$(GOOS)_$(GOARCH)
ARCHIVE_DIR := $(DIST_DIR)/$(ARCHIVE_NAME)
ARCHIVE := $(DIST_DIR)/$(ARCHIVE_NAME).tar.gz

LDFLAGS := -s -w
LDFLAGS += -X github.com/takayoshiotake/shiroyagi/internal/version.Version=$(SHIROYAGI_VERSION)
LDFLAGS += -X github.com/takayoshiotake/shiroyagi/internal/version.Commit=$(SHIROYAGI_COMMIT)

.PHONY: build package checksums release clean

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -buildvcs=true -ldflags="$(LDFLAGS)" \
		-o $(APP) ./cmd/shiroyagi
	go version -m $(APP) > $(BUILD_INFO)

package: build
	rm -rf $(ARCHIVE_DIR)
	mkdir -p $(ARCHIVE_DIR)
	cp $(APP) $(BUILD_INFO) $(ARCHIVE_DIR)/
	tar -czf $(ARCHIVE) -C $(DIST_DIR) $(ARCHIVE_NAME)

checksums: package
	cd $(DIST_DIR) && shasum -a 256 $(ARCHIVE_NAME).tar.gz > checksums.txt

release: checksums

clean:
	rm -f $(APP) $(BUILD_INFO)
	rm -rf $(DIST_DIR)
