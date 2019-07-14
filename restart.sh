#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname -s)-config.json

$KILL_CMD go-dns

sleep 2

if [ -f logs/output ]; then
 mv logs/output logs/output.1
fi

nohup ./go-dns $CONFIG_FILE 2>&1 > logs/output &
