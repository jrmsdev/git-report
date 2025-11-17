#!/bin/bash
exec git log --numstat --pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00
