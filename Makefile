Version := $(shell git describe --tags --dirty)
GitCommit := $(shell git rev-parse HEAD)
LDFLAGS := "-X main.version=$(Version) -X main.commit=$(GitCommit)"

build:
	go build -ldflags $(LDFLAGS) -o bin/vals ./cmd/vals

install: build
	mv bin/vals ~/bin/
