#!/bin/sh

BLOCKLIST='blocklist/blocklist.txt'

rm -f $BLOCKLIST

echo 'corp.target.com' >> $BLOCKLIST
echo 'dist.target.com' >> $BLOCKLIST
echo 'labs.target.com' >> $BLOCKLIST
echo 'hq.target.com' >> $BLOCKLIST
echo 'prod.target.com' >> $BLOCKLIST
echo 'stores.target.com' >> $BLOCKLIST
echo 'target.com.target.com' >> $BLOCKLIST
echo '_udp.target.com' >> $BLOCKLIST

curl --silent 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts' | grep '^0\.0\.0\.0' | grep -v '0\.0\.0\.0$' | sort -u | awk '{print $2}' >> $BLOCKLIST

wc -l $BLOCKLIST
