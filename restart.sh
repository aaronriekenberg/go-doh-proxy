#!/bin/sh -x

KILL_CMD=pkill

$KILL_CMD go-dns

sleep 2

nohup ./go-dns 192.168.1.1:10053 2>&1 | svlogd logs &
