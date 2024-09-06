Version := $(shell git describe --tags --dirty --always)
PKGS    := $(shell go list ./... | grep -v /vendor/)
GitCommit := $(shell git rev-parse HEAD)
INITIAL_LDFLAGS := -X main.version=$(Version) -X main.commit=$(GitCommit)

platform := $(shell uname -o)

ifeq ($(platform),Darwin)
		CC_FLAGS := CC=clang CXX=clang++
		LDFLAGS := '$(INITIAL_LDFLAGS)'
endif

ifeq ($(platform),GNU/Linux)
		CC_FLAGS := CC=musl-gcc
		LDFLAGS := '$(INITIAL_LDFLAGS) -linkmode external -extldflags "-static -Wl,-unresolved-symbols=ignore-all"'
endif

build:
	CGO_ENABLED=1 $(CC_FLAGS) go build -ldflags $(LDFLAGS) -o bin/vals ./cmd/vals

install: build
	mv bin/vals ~/bin/

lint:
	golangci-lint run -v --out-format=github-actions

test:
	CGO_ENABLED=1 $(CC_FLAGS) go test -ldflags=$(LDFLAGS) -v ${PKGS} -coverprofile cover.out -race -p=1
	go tool cover -func cover.out
.PHONY: test
