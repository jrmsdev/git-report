# git-report: Git Repository Contribution Report Tool

## Overview
A command-line tool written in Go that parses `git log` output and generates a SQLite database for querying repository contributions via Datasette.

## Purpose
Generate contributor reports by analyzing git history, including commit metadata and file change patterns to track contributions to specific parts of a repository.

## CLI Interface

### Basic usage
```bash
git-report /path/to/repo -o output.db
```

### Optional flags
- `-o, --output`: output database file path (default: `repo.db`)
- `--since`: start date filter
- `--until`: end date filter
- `--author`: filter by author pattern
- `--path`: filter by file path pattern
- `--branch`: specify branch (default: current branch)
