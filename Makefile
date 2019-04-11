BIN := gojq
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-X github.com/itchyny/$(BIN)/cli.revision=$(CURRENT_REVISION)"
export GO111MODULE=on

.PHONY: all
all: clean build

.PHONY: build
build:
	go build -ldflags=$(BUILD_LDFLAGS) -o build/$(BIN) ./cmd/$(BIN)

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: cross
cross: crossdeps
	goxz -n $(BIN) -build-ldflags=$(BUILD_LDFLAGS) ./cmd/$(BIN)

.PHONY: crossdeps
crossdeps:
	GO111MODULE=off go get github.com/Songmu/goxz/cmd/goxz

.PHONY: test
test: build
	go test -v ./...

.PHONY: lint
lint: lintdeps
	go vet ./...
	golint -set_exit_status ./...

.PHONY: lintdeps
lintdeps:
	GO111MODULE=off go get golang.org/x/lint/golint

.PHONY: clean
clean:
	rm -rf build goxz
	go clean
