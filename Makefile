BIN := gojq
VERSION := $$(make -s show-version)
VERSION_PATH := cli
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-s -w -X github.com/itchyny/$(BIN)/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin
SHELL := /bin/bash
export GO111MODULE=on

.PHONY: all
all: build

.PHONY: build
build:
	go build -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

.PHONY: build-dev
build-dev: parser.go builtin.go
	go build -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

.PHONY: build-debug
build-debug: parser.go builtin.go
	go build -tags debug -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

builtin.go: builtin.jq parser.go.y parser.go query.go operator.go _tools/*
	GOOS= GOARCH= go generate

.SUFFIXES:
parser.go: parser.go.y lexer.go $(GOBIN)/goyacc
	goyacc -o $@ $<

$(GOBIN)/goyacc:
	@cd && go get golang.org/x/tools/cmd/goyacc

.PHONY: install
install:
	go install -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: install-dev
install-dev: parser.go builtin.go
	go install -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: install-debug
install-debug: parser.go builtin.go
	go install -tags debug -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: show-version
show-version: $(GOBIN)/gobump
	@gobump show -r $(VERSION_PATH)

$(GOBIN)/gobump:
	@cd && go get github.com/x-motemen/gobump/cmd/gobump

.PHONY: cross
cross: $(GOBIN)/goxz CREDITS
	build() { \
		goxz -n $(BIN) -pv=v$(VERSION) -os=$$1 -arch=$$2 \
			-include _$(BIN) -build-ldflags=$(BUILD_LDFLAGS) ./cmd/$(BIN); \
	}; \
	build linux,darwin,windows amd64 && build linux,darwin arm64

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
lint: $(GOBIN)/golint
	go vet ./...
	golint -set_exit_status ./...

$(GOBIN)/golint:
	cd && go get golang.org/x/lint/golint

.PHONY: maligned
maligned: $(GOBIN)/maligned
	! maligned . 2>&1 | grep -v pointer | grep ^

$(GOBIN)/maligned:
	cd && go get github.com/mdempsky/maligned

.PHONY: check-tools
check-tools:
	go run _tools/print_builtin.go

.PHONY: clean
clean:
	rm -rf $(BIN) goxz CREDITS
	go clean

.PHONY: update
update:
	export GOPROXY=direct
	rm -f go.sum && go get -u -d ./... && go get github.com/mattn/go-runewidth@v0.0.9 && go mod tidy
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
