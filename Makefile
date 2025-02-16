Version := $(shell git describe --tags --dirty --always)
PKGS    := $(shell go list ./... | grep -v /vendor/)
GitCommit := $(shell git rev-parse HEAD)
LDFLAGS := "-X main.version=$(Version) -X main.commit=$(GitCommit)"

build:
	go build -ldflags $(LDFLAGS) -o bin/vals ./cmd/vals

install: build
	mv bin/vals ~/bin/

lint:
	golangci-lint run -v --out-format=github-actions


test:
	go test -v ${PKGS} -coverprofile cover.out -race -p=1
	go tool cover -func cover.out
.PHONY: test
