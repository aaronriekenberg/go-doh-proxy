GOCMD=go
GOBUILD=$(GOCMD) build

BINARY_NAME=go-doh-proxy
BINARY_NAME_LINUX_AMD64=go-doh-proxy-linux-amd64

GIT_COMMIT := $(shell git rev-parse HEAD)

all: build build-linux-amd64

build:
	$(GOBUILD) -o $(BINARY_NAME) -ldflags="-X main.gitCommit=$(GIT_COMMIT)"

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME_LINUX_AMD64) -ldflags="-X main.gitCommit=$(GIT_COMMIT)"
