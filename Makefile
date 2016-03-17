all: bin/bootkube

bin/bootkube: cmd/*.go pkg/bootkube/*.go
	mkdir -p bin
	GO15VENDOREXPERIMENT=1 CGO_ENABLED=0 go build -o bin/bootkube cmd/main.go

clean:
	rm -rf bin

.PHONY: all clean

