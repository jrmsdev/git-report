#!/bin/bash
set -eu
make install
~/go/bin/git-report -v
exec datasette -h 0.0.0.0 report.db
