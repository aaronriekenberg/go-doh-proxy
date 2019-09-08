#!/bin/sh

pgrep go-dns-proxy > /dev/null 2>&1
if [ $? -eq 1 ]; then
  cd ~/go-dns-proxy
  ./restart.sh > /dev/null 2>&1
fi
