#!/bin/bash
set -eu
make install
exec ~/go/bin/git-report "$@"
