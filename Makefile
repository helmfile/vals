build:
	go build -o bin/vals ./cmd/vals

install: build
	mv bin/vals ~/bin/
