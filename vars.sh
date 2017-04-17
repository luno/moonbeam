#!/bin/bash

export GOPATH=`pwd`

mkdir -p src/github.com/luno
rm -f src/github.com/luno/moonbeam
ln -s ../../.. src/github.com/luno/moonbeam
