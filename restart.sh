#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname -s)-config.json

$KILL_CMD go-doh-proxy

sleep 2

export PATH=${HOME}/bin:$PATH

nohup ./go-doh-proxy $CONFIG_FILE 2>&1 | simplerotate logs &
