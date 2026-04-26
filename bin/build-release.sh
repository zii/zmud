#!/bin/bash

set -e

# zmud v0.3 三平台打包发布脚本

rm -rf dist
mkdir -p dist

echo "=== Building for darwin/amd64 (Intel Mac) ==="
GOOS=darwin GOARCH=amd64 go build -o dist/zmud-darwin-amd64 zmud/cmd

echo "=== Building for linux/amd64 ==="
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    CC="zig cc -target x86_64-linux-gnu" \
    CXX="zig c++ -target x86_64-linux-gnu" \
    go build -o dist/zmud-linux-amd64 zmud/cmd

echo "=== Building for windows/amd64 ==="
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
    CC="zig cc -target windows-x86_64" \
    CXX="zig c++ -target windows-x86_64" \
    go build -o dist/zmud-windows-amd64.exe zmud/cmd

echo ""
echo "=== Build complete ==="
ls -lh dist/
