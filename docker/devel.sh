#!/bin/sh
set -eu
exec docker run -it --rm -u devel \
	--name git-report \
	--hostname git-report.local \
	-v "${PWD}:/opt/src" \
	--workdir /opt/src \
    -p 127.0.0.1:8001:8001 \
	jrmsdev/git-report
