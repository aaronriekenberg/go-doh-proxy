#!/bin/bash

export GIT_COMMIT=$(git rev-parse HEAD)
go build -x -ldflags "-X main.GitCommit=$GIT_COMMIT"
