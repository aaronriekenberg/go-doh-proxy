#!/bin/sh

pgrep go-doh-proxy > /dev/null 2>&1
if [ $? -eq 1 ]; then
  cd ~/go-doh-proxy
  ./restart.sh > /dev/null 2>&1
fi
