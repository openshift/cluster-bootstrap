export GO15VENDOREXPERIMENT:=1
export CGO_ENABLED:=0

GOFILES:=$(shell find . -path ./vendor -prune -type f -o -name '*.go')
GOPACKAGES:=$(shell go list ./... | grep -v '/vendor/')

all: bin/bootkube

vet:
	@go vet $(GOPACKAGES)

bin/bootkube: $(GOFILES)
	mkdir -p bin
	go generate pkg/assets/assets.go
	go build -o bin/bootkube github.com/coreos/bootkube/cmd/bootkube

clean:
	rm bin/bootkube
	rm pkg/assets/internal/*.go

.PHONY: all clean vet

