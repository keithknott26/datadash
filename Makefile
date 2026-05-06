.PHONY: build test clean

BINARY := datadash

build:
	go build -o $(BINARY) ./cmd

test:
	go test -v -race -coverprofile=coverage.out ./...

clean:
	rm -f $(BINARY) coverage.out