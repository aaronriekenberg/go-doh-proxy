#!/bin/sh

pgrep go-dns > /dev/null 2>&1
if [ $? -eq 1 ]; then
  cd ~/go-dns
  ./restart.sh > /dev/null 2>&1
fi
