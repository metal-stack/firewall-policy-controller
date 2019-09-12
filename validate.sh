#!/bin/sh

set -e

echo "validate expected.nftable items in test_data directory"
cd test_data
for i in $(find . -type f -name expected.nftablev4); do
    echo "validating: $i"
    nft -c -f "$PWD/$i"
done

for i in $(find . -type f -name expected.nftablev6); do
    echo "validating: $i"
    nft -c -f "$PWD/$i"
done