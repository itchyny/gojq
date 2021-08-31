BIN := gojq
VERSION := $$(make -s show-version)
VERSION_PATH := cli
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-s -w -X github.com/itchyny/$(BIN)/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin
SHELL := /bin/bash

.PHONY: all
all: build

.PHONY: build
build:
	go build -tags forceposix -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

.PHONY: build-dev
build-dev: parser.go builtin.go
	go build -tags forceposix -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

.PHONY: build-debug
build-debug: parser.go builtin.go
	go build -tags forceposix,debug -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

builtin.go: builtin.jq parser.go.y parser.go query.go operator.go _tools/*
	GOOS= GOARCH= go generate

.SUFFIXES:
parser.go: parser.go.y $(GOBIN)/goyacc
	goyacc -o $@ $<

$(GOBIN)/goyacc:
	@cd && go get golang.org/x/tools/cmd/goyacc

.PHONY: install
install:
	go install -tags forceposix -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: install-dev
install-dev: parser.go builtin.go
	go install -tags forceposix -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: install-debug
install-debug: parser.go builtin.go
	go install -tags forceposix,debug -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: show-version
show-version: $(GOBIN)/gobump
	@gobump show -r $(VERSION_PATH)

$(GOBIN)/gobump:
	@cd && go get github.com/x-motemen/gobump/cmd/gobump

.PHONY: cross
cross: $(GOBIN)/goxz CREDITS
	goxz -n $(BIN) -pv=v$(VERSION) -arch=amd64,arm64 \
		-build-ldflags=$(BUILD_LDFLAGS) ./cmd/$(BIN)

$(GOBIN)/goxz:
	cd && go get github.com/Songmu/goxz/cmd/goxz

CREDITS: $(GOBIN)/gocredits go.sum
	go mod tidy
	gocredits -w .

$(GOBIN)/gocredits:
	cd && go get github.com/Songmu/gocredits/cmd/gocredits

.PHONY: test
test: build
	go test -v -race ./...

.PHONY: lint
lint: $(GOBIN)/staticcheck
	go vet ./...
	staticcheck -tags debug ./...

$(GOBIN)/staticcheck:
	cd && go get honnef.co/go/tools/cmd/staticcheck

.PHONY: fieldalignment
fieldalignment: $(GOBIN)/fieldalignment
	! fieldalignment . 2>&1 | grep -v pointer | grep ^

$(GOBIN)/fieldalignment:
	cd && go get golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment

.PHONY: check-tools
check-tools:
	go run _tools/print_builtin.go

.PHONY: clean
clean:
	rm -rf $(BIN) goxz CREDITS
	go clean

.PHONY: update
update: export GOPROXY=direct
update:
	rm -f go.sum && go get -u -d ./... && go get -d github.com/mattn/go-runewidth@v0.0.9 && go mod tidy
	sed -i.bak '/require (/,/)/d' go.dev.mod && rm -f go.dev.{sum,mod.bak}
	go get -u -d -modfile=go.dev.mod github.com/itchyny/{astgen,timefmt}-go && go generate

.PHONY: bump
bump: $(GOBIN)/gobump
ifneq ($(shell git status --porcelain),)
	$(error git workspace is dirty)
endif
ifneq ($(shell git rev-parse --abbrev-ref HEAD),main)
	$(error current branch is not main)
endif
	@gobump up -w "$(VERSION_PATH)"
	git commit -am "bump up version to $(VERSION)"
	git tag "v$(VERSION)"
	git push origin main
	git push origin "refs/tags/v$(VERSION)"

.PHONY: upload
upload: $(GOBIN)/ghr
	ghr "v$(VERSION)" goxz

$(GOBIN)/ghr:
	cd && go get github.com/tcnksm/ghr
