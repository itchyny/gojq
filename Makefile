BIN := gojq
VERSION := $$(make -s show-version)
VERSION_PATH := cli
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

.PHONY: show-version
show-version:
	@GO111MODULE=off go get github.com/motemen/gobump/cmd/gobump
	@gobump show -r $(VERSION_PATH)

.PHONY: cross
cross: crossdeps
	goxz -n $(BIN) -pv=v$(VERSION) -build-ldflags=$(BUILD_LDFLAGS) ./cmd/$(BIN)

.PHONY: crossdeps
crossdeps:
	GO111MODULE=off go get github.com/Songmu/goxz/cmd/goxz

.PHONY: test
test: build
	go test -v ./...

.PHONY: lint
lint: lintdeps
	golint -set_exit_status ./...

.PHONY: lintdeps
lintdeps:
	GO111MODULE=off go get golang.org/x/lint/golint

.PHONY: clean
clean:
	rm -rf build goxz
	go clean

.PHONY: bump
bump:
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
upload:
	GO111MODULE=off go get github.com/tcnksm/ghr
	ghr "v$(VERSION)" goxz

.PHONY: release
release: test lint clean bump crossdocker upload
