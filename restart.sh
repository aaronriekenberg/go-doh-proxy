#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname -s)-config.json

$KILL_CMD go-dns

sleep 2

export PATH=${HOME}/bin:$PATH

nohup ./go-dns $CONFIG_FILE 2>&1 | svlogd logs &
