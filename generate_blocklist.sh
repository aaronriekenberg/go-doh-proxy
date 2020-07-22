#!/bin/sh

rm -f blocklist.txt

echo 'corp.target.com' >> blocklist.txt
echo 'dist.target.com' >> blocklist.txt
echo 'labs.target.com' >> blocklist.txt
echo 'hq.target.com' >> blocklist.txt
echo 'prod.target.com' >> blocklist.txt
echo 'stores.target.com' >> blocklist.txt
echo 'target.com.target.com' >> blocklist.txt
echo '_udp.target.com' >> blocklist.txt

curl --silent 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts' | grep '^0\.0\.0\.0' | sort -u | awk '{print $2}' >> blocklist.txt

wc -l blocklist.txt