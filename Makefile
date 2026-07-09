APP := shiroyagi
BUILD_INFO := $(APP)-build-info.txt

.PHONY: build

build:
	go build -trimpath -buildvcs=true -ldflags="-s -w" -o $(APP) ./cmd/shiroyagi
	go version -m $(APP) > $(BUILD_INFO)
