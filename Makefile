VERSION   ?= $(shell git describe --tags --always)
BUILDTIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
CODENAME  ?= percent-of-the-time-it-works-everytime
MAIN_PACKAGE = main

LDFLAGS := -s -w -buildid= \
	-X $(MAIN_PACKAGE).version=$(VERSION) \
	-X $(MAIN_PACKAGE).codename=$(CODENAME) \
	-X $(MAIN_PACKAGE).buildTime=$(BUILDTIME)

.PHONY: build release

build:
	CGO_ENABLED=0 go build \
	  -trimpath \
	  -ldflags="$(LDFLAGS)" \
	  -o dist/merton \
	  ./cmd/merton

release-linux-amd64:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/merton-linux-amd64       ./cmd/merton

release-linux-arm64:
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/merton-linux-arm64       ./cmd/merton

release-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/merton-windows-amd64.exe ./cmd/merton

release-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/merton-darwin-arm64      ./cmd/merton

release-all: release-linux-amd64 release-linux-arm64 release-windows-amd64 release-darwin-arm64