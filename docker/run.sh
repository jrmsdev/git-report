#!/bin/bash
set -eu
mkdir -vp ./build
make install
exec "${GOPATH}/bin/git-report" "$@"
