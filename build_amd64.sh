#!/bin/bash

export GIT_COMMIT=$(git rev-parse HEAD)
GOOS=linux GOARCH=amd64 go build -x -ldflags "-X main.GitCommit=$GIT_COMMIT"
