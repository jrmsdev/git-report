#!/bin/bash
set -eu
mkdir -vp ./build
make install
~/go/bin/git-report -v
exec datasette -h 0.0.0.0 build/report.db -m build/report-metadata.json
