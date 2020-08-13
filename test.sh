#!/bin/bash
GOPATH=${GOPATH:-$HOME/go}
KASPAD_PKG="github.com/kaspanet/kaspad"
KASPAD_DIR="$GOPATH/src/$KASPAD_PKG"
SEEDER_DIR=$PWD
BRANCH=$(git branch --show-current)

echo $KASPAD_DIR

set -e

if [ ! -d "$KASPAD_DIR" ]; then
  git clone $KASPAD_PKG
fi

cd $KASPAD_DIR
git checkout $BRANCH
go build
./kaspad --datadir=$SEEDER_DIR/testdata/kaspad1 --allowlocalpeers --notls --rpcuser=test --rpcpass=test --listen=127.0.0.1:16621 --rpclisten=127.0.0.1:16620 --devnet --grpcseed=127.0.0.1:3737 &
KASPAD1_PID=$!
cd -
sleep 1

go build
./dnsseeder -n test.com -H test.com -s 127.0.0.1 --devnet -p "127.0.0.1:16621" &
SEEDER_PID=$!
sleep 3

cd $KASPAD_DIR
./kaspad --datadir=$SEEDER_DIR/testdata/kaspad2 --allowlocalpeers --notls --rpcuser=test --rpcpass=test --listen=127.0.0.1:16631 --rpclisten=127.0.0.1:16630 --devnet --grpcseed=127.0.0.1:3737 &
KASPAD2_PID=$!
cd -
sleep 2

RESULT=$(go run cmd/get_peers_list.go)
EXPECTED="127.0.0.1:16621,127.0.0.1:16611"

sleep 2
kill $KASPAD1_PID $KASPAD2_PID $SEEDER_PID
rm -rf testdata

if [ $RESULT != $EXPECTED ]; then
  echo "Test failed: Unexpected list addresses: " $RESULT
  exit 1
fi