#!/bin/bash
set -eu
make install
~/go/bin/git-report "$@"
exec datasette -h 0.0.0.0 report.db
