BIN := gojq
VERSION := $$(make -s show-version)
VERSION_PATH := cli
CURRENT_REVISION := $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS := "-s -w -X github.com/itchyny/$(BIN)/cli.revision=$(CURRENT_REVISION)"
GOBIN ?= $(shell go env GOPATH)/bin
export GO111MODULE=on

.PHONY: all
all: clean build

.PHONY: build
build: builtin_gen.go
	go build -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

.PHONY: build-debug
build-debug: builtin_gen.go
	go build -tags debug -ldflags=$(BUILD_LDFLAGS) -o $(BIN) ./cmd/$(BIN)

builtin_gen.go: builtin.go query.go operator.go
	rm -f $@
	GOOS= GOARCH= go generate

.PHONY: install
install: builtin_gen.go
	go install -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: install-debug
install-debug: builtin_gen.go
	go install -tags debug -ldflags=$(BUILD_LDFLAGS) ./...

.PHONY: show-version
show-version: $(GOBIN)/gobump
	@gobump show -r $(VERSION_PATH)

$(GOBIN)/gobump:
	@GO111MODULE=off go get github.com/motemen/gobump/cmd/gobump

.PHONY: cross
cross: $(GOBIN)/goxz
	goxz -n $(BIN) -pv=v$(VERSION) -build-ldflags=$(BUILD_LDFLAGS) ./cmd/$(BIN)

$(GOBIN)/goxz:
	GO111MODULE=off go get github.com/Songmu/goxz/cmd/goxz

.PHONY: test
test: build
	go test -v ./...

.PHONY: lint
lint: $(GOBIN)/golint
	golint -set_exit_status ./...

$(GOBIN)/golint:
	GO111MODULE=off go get golang.org/x/lint/golint

.PHONY: check-tools
check-tools:
	ls _tools/* | xargs -L1 -I% sh -c 'go run % >/dev/null 2>&1 || go run %'

.PHONY: clean
clean:
	rm -rf $(BIN) goxz
	go clean

.PHONY: bump
bump: $(GOBIN)/gobump
	@git status --porcelain | grep "^" && echo "git workspace is dirty" >/dev/stderr && exit 1 || :
	gobump set $(shell sh -c 'read -p "input next version (current: $(VERSION)): " v && echo $$v') -w $(VERSION_PATH)
	git commit -am "bump up version to $(VERSION)"
	git tag "v$(VERSION)"
	git push
	git push --tags

.PHONY: crossdocker
crossdocker:
	docker run --rm -v `pwd`:"/$${PWD##*/}" -w "/$${PWD##*/}" golang make cross

.PHONY: upload
upload: $(GOBIN)/ghr
	ghr "v$(VERSION)" goxz

$(GOBIN)/ghr:
	GO111MODULE=off go get github.com/tcnksm/ghr

.PHONY: release
release: test lint clean bump crossdocker upload
