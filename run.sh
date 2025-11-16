#!/bin/bash
set -eu
make -s build
exec build/git-report "$@"
