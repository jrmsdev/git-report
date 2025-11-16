# git-report: Git Repository Contribution Report Tool

## Overview
A command-line tool written in Go that parses `git log` output and generates a
SQLite database for querying repository contributions via Datasette.

## Purpose
Generate contributor reports by analyzing git history, including commit metadata
and file change patterns to track contributions to specific parts of a
repository.

## Architecture

### Core Flow
1. Execute `git log` commands with structured output formatting
2. Parse output into normalized data structures
3. Write parsed data to SQLite database
4. Output `.db` file for consumption by Datasette

### Technology Stack
- **Language**: Go (for performance and single-binary distribution)
- **Database**: SQLite3
- **Query/Visualization**: Datasette (separate tool, consumes generated .db
  file)

## Database Schema

### `commits` table
- `hash` (TEXT, PRIMARY KEY): commit SHA
- `author` (TEXT): author name
- `email` (TEXT): author email
- `date` (DATETIME): commit timestamp
- `message` (TEXT): commit message

### `file_changes` table
- `id` (INTEGER, PRIMARY KEY)
- `commit_hash` (TEXT, FOREIGN KEY): references commits(hash)
- `filepath` (TEXT): path to changed file
- `additions` (INTEGER): lines added
- `deletions` (INTEGER): lines deleted
- `change_type` (TEXT): 'A' (added), 'M' (modified), 'D' (deleted),
  'R' (renamed)

### Optional: `patterns` or `components` table
Pre-categorized path patterns for common query scenarios (can be added later)

## Git Log Integration

### Required git log flags
- `--numstat`: get per-file addition/deletion statistics
- `--name-status`: get file operation types (A/M/D/R)
- `--pretty=format:...`: structured commit metadata output
- Optional filters: `--since`, `--until`, `--author`, `-- <path-pattern>`

### Suggested git log format
```
--pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00 --numstat --name-status
```
(Using null bytes `%x00` as field delimiters for reliable parsing)

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

## File Pattern Analysis Strategy

### At parse time
- Extract all changed files per commit
- Store raw filepath in `file_changes` table

### At query time (via Datasette)
- Use SQL to filter/group by path patterns:
  ```sql
  SELECT author, COUNT(*)
  FROM commits c
  JOIN file_changes fc ON c.hash = fc.commit_hash
  WHERE fc.filepath LIKE 'src/api/%'
  GROUP BY author
  ```
- Datasette provides web UI and JSON API for queries
- Can add custom views/plugins for common reporting patterns

## Implementation Notes

### Go packages needed
- `os/exec`: execute git commands
- `database/sql` + `github.com/mattn/go-sqlite3`: SQLite operations
- `encoding/csv` or custom parser: parse git log output
- `flag` or `github.com/spf13/cobra`: CLI argument parsing

### Error handling considerations
- Validate repo path exists and is a git repository
- Handle git command failures gracefully
- Ensure database writes are atomic/transactional
- Validate parsed data before insertion

### Performance optimizations
- Use transactions for bulk inserts
- Prepared statements for repeated inserts
- Stream processing for large repos (don't load all data into memory)

## Datasette Integration

### No direct integration needed
- Tool outputs standalone `.db` file
- User runs separately: `datasette repo.db`
- Datasette provides:
  - Web-based query interface
  - JSON API
  - Export capabilities (CSV, JSON)
  - Plugin ecosystem for custom visualizations

### Example Datasette queries
- Top contributors by commit count
- File change frequency heatmaps
- Contribution timelines
- Component ownership analysis (via path filtering)

## Future Enhancements
- Pre-built SQL views for common reports
- Datasette metadata.json generation for UI customization
- Support for multiple repos in single database
- Branch comparison capabilities
- Merge commit handling options
