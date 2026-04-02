VERSION ?= $(shell git describe --tags --always)
BUILD_TIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
MAIN_PACKAGE = main

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
	  -trimpath \
	  -ldflags="-s -w -buildid= -X $(MAIN_PACKAGE).version=$(VERSION) -X $(MAIN_PACKAGE).buildTime=$(BUILD_TIME)" \
	  -o dist/merton \
	  ./cmd/merton