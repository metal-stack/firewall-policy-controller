#!/bin/sh

set -e

iface0=$(ip link show | grep -v "link/" | grep -v lo | head -n 1 | cut -d " " -f2 | cut -d ":" -f1)
iface1=$(ip link show | grep -v "link/" | grep -v lo | tail -n +2 | head -n 1 | cut -d " " -f2 | cut -d ":" -f1)
echo "validate expected.nftable items in test_data directory"
for i in $(find . -type f -name expected.nftablev4); do
    echo "validating: $i"
    sed s/lan0/"${iface0}"/g --in-place "$PWD/$i"
    sed s/lan1/"${iface1}"/g --in-place "$PWD/$i"
    nft -c -f "$PWD/$i"
done

for i in $(find . -type f -name expected.nftablev6); do
    echo "validating: $i"
    sed s/lan0/"${iface0}"/g --in-place "$PWD/$i"
    sed s/lan1/"${iface1}"/g --in-place "$PWD/$i"
    nft -c -f "$PWD/$i"
done