#!/bin/sh -x

KILL_CMD=pkill

$KILL_CMD go-dns

sleep 2

nohup ./go-dns 2>&1 | svlogd logs &
