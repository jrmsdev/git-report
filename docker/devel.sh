#!/bin/sh
set -eu
exec docker run -it --rm -u devel \
	--name git-report \
	--hostname git-report.local \
	-v "${PWD}:/opt/src:ro" \
	--workdir /opt/src \
	jrmsdev/git-report
