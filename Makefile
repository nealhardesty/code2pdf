GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOTEST=$(GOCMD) test

.PHONY: build run clean mod test
default: build

increment-version:
	@echo 'package main' > version.go
	@echo 'const Version = "'$$(date +%Y%m%d%H%M%S)'"' >> version.go
	@echo "Updated version to $$(date +%Y%m%d%H%M%S)"

build: increment-version
	$(GOBUILD) .
run:
	go run .

clean:
	$(GOCLEAN)

mod:
	$(GOMOD) tidy
	$(GOMOD) vendor

test:
	$(GOTEST) -v ./...

install:
	go install .