#!/bin/bash

set -e

case "$1" in
1)
	go build -o zmud zmud/cmd
	mv zmud /usr/local/bin
	zmud
	;;
2)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		CC="zig cc -target x86_64-linux-gnu" \
		CXX="zig c++ -target x86_64-linux-gnu" \
		go build -o zmud zmud/cmd
	rsync -rutz ./zmud root@sg1:/usr/local/bin
	;;
*)
	echo "Usage: $0 [1|2]"
	echo "  1: build and run locally"
	echo "  2: upload to sg1"
	;;
esac
