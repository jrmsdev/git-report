# git-report: Git Repository Contribution Report Tool

## Overview
A command-line tool written in Go that parses `git log` output from multiple repositories and generates a SQLite database for querying project contributions via Datasette.

## Purpose
Generate contributor reports by analyzing git history across one or more repositories, including commit metadata and file change patterns to track contributions to specific components of a project.

## Architecture

### Core Flow
1. Read configuration file specifying repositories and report parameters
2. Execute `git log` commands for each repository with structured output
3. Parse output into normalized data structures
4. Write parsed data to SQLite database
5. Output `.db` file for consumption by Datasette

### Technology Stack
- **Language**: Go (for performance and single-binary distribution)
- **Database**: SQLite3
- **Query/Visualization**: Datasette (separate tool, consumes generated .db file)
- **Dependencies**: Keep them minimal

## Configuration File

### Format
YAML configuration file specifying repositories and report parameters.

### Example (YAML)
```yaml
output: project-report.db

repositories:
  - path: /path/to/backend-repo
    name: backend
  - path: /path/to/frontend-repo
    name: frontend

filters:
  since: 2024-01-01
  until: 2025-12-31
  authors:
    - john@example.com
    - jane@example.com
  branch: main

components:
  - name: API
    paths:
      - backend:src/api/**
      - backend:internal/handlers/**
  - name: Frontend UI
    paths:
      - frontend:src/components/**
      - frontend:src/pages/**
  - name: Database
    paths:
      - backend:migrations/**
      - backend:internal/models/**
```

### Configuration Fields

#### `output` (string)
Path to output database file (default: `report.db`)

#### `repositories` (array)
- `path` (string, required): absolute or relative path to git repository
- `name` (string, required): identifier for the repository

#### `filters` (object, optional)
- `since` (string): start date (YYYY-MM-DD format)
- `until` (string): end date (YYYY-MM-DD format)
- `authors` (array of strings): filter by author emails or patterns
- `branch` (string): branch to analyze (default: current branch)

#### `components` (array, optional)
- `name` (string, required): component identifier
- `paths` (array of strings, required): path patterns in format `repo_name:path/pattern`
  - Supports glob patterns: `**` (recursive), `*` (single level)
  - Examples: `backend:src/api/**`, `frontend:*.ts`

## Database Schema

### `repositories` table
- `id` (INTEGER, PRIMARY KEY AUTOINCREMENT)
- `name` (TEXT, UNIQUE): repository name from config
- `path` (TEXT): filesystem path

### `commits` table
- `hash` (TEXT, PRIMARY KEY): commit SHA
- `repository_id` (INTEGER, FOREIGN KEY): references repositories(id)
- `author` (TEXT): author name
- `email` (TEXT): author email
- `date` (DATETIME): commit timestamp
- `message` (TEXT): commit message

### `file_changes` table
- `id` (INTEGER, PRIMARY KEY AUTOINCREMENT)
- `commit_hash` (TEXT, FOREIGN KEY): references commits(hash)
- `filepath` (TEXT): path to changed file
- `additions` (INTEGER): lines added
- `deletions` (INTEGER): lines deleted
- `change_type` (TEXT): 'A' (added), 'M' (modified), 'D' (deleted), 'R' (renamed)

### `components` table
- `id` (INTEGER, PRIMARY KEY AUTOINCREMENT)
- `name` (TEXT, UNIQUE): component name from config
- `path_patterns` (TEXT): JSON array of path patterns

### `component_contributions` table
Pre-computed statistics for efficient querying:
- `id` (INTEGER, PRIMARY KEY AUTOINCREMENT)
- `component_id` (INTEGER, FOREIGN KEY): references components(id)
- `repository_id` (INTEGER, FOREIGN KEY): references repositories(id)
- `author` (TEXT)
- `email` (TEXT)
- `commit_count` (INTEGER)
- `total_additions` (INTEGER)
- `total_deletions` (INTEGER)

### Indexes
- `idx_commits_repo` on commits(repository_id)
- `idx_file_changes_commit` on file_changes(commit_hash)
- `idx_component_contributions_component` on component_contributions(component_id)

## Git Log Integration

### Required git log flags
- `--numstat`: get per-file addition/deletion statistics
- `--pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00`: structured commit metadata
- Filters from config: `--since`, `--until`, `--author`, branch name

### Git log format
```
--pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00 --numstat
```

Fields separated by null bytes (`%x00`):
- `%H`: commit hash
- `%an`: author name
- `%ae`: author email
- `%ai`: author date (ISO 8601)
- `%s`: subject (commit message)
- `%x00`: null byte delimiter (final one ends the commit header line)

### Git log output format
Each commit consists of:
1. Header line with null-byte-separated fields
2. Followed by `--numstat` lines (one per file changed)
3. Empty line separator between commits

### Parsing implementation
- Lines containing `\x00` are commit header lines
- Lines after header are `--numstat` output until empty line or next commit
- `--numstat` format: `<additions><tab><deletions><tab><filepath>`
- Binary files: `-	-	<filepath>` (skipped)
- Renames: `0	0	old/path => new/path` (takes new path, marks as 'R')
- Change type inference:
  - 'R': rename (detected by `=>` in filepath, specifically looking for pattern with spaces around `=>`)
  - 'A': addition (additions > 0, deletions = 0)
  - 'D': deletion (additions = 0, deletions > 0)
  - 'M': modification (all other cases)

## CLI Interface

### Basic usage
```bash
git-report [config.yaml]
```

If no config file is specified, defaults to `report.yaml`.

### Optional flags
- `-c <path>`, `--config <path>`: path to configuration file
- `-v`, `--verbose`: verbose output (shows repository processing and match counts)
- `--dry-run`: validate config without generating report

### Flag handling
- Positional argument (first non-flag argument) overrides `-c`/`--config`
- Either `-v` or `--verbose` enables verbose mode
- Either `-c` or `--config` works

## Component Analysis

### At parse time
1. Load component definitions from config
2. Insert components into database with JSON-encoded path patterns
3. Parse all commits and file changes from each repository
4. After parsing, compute component contributions by:
   - Iterating through each component
   - Parsing path patterns to extract repo name and path pattern
   - Querying all file changes for matching repository
   - Applying custom pattern matching to file paths
   - Aggregating statistics (unique commits, additions, deletions) per author

### Path pattern matching
Custom `matchPath()` function supporting:
- **Exact match**: path equals pattern exactly
- **`**` patterns** (recursive directory matching):
  - `**/something`: matches if path ends with "something" or contains "/something"
  - `something/**`: matches if path starts with "something/" or equals "something"
  - `prefix/**/suffix`: matches if path starts with prefix and ends with suffix
  - `**` alone: matches everything
- **`*` patterns** (single-level wildcard):
  - Uses `filepath.Match()` for patterns containing `*` but not `**`
  - Matches within single directory level only

Pattern matching is case-sensitive and matches against full file paths relative to repository root.

### Component contribution computation
Implemented as an in-memory aggregation:
- Creates a map keyed by (component_id, repository_id, email)
- Tracks unique commit hashes per author using a set
- Accumulates additions and deletions
- Writes aggregated results to `component_contributions` table in a single transaction

## Implementation Notes

### Go packages used
- `os/exec`: execute git commands
- `database/sql` + `github.com/mattn/go-sqlite3`: SQLite operations
- `gopkg.in/yaml.v3`: YAML config parsing
- `flag`: CLI argument parsing
- `encoding/json`: JSON encoding for component path patterns
- `bufio`: streaming line-by-line parsing
- `path/filepath`: used in single-wildcard pattern matching
- `strings`: string manipulation
- `time`: timestamp parsing

### Error handling
- Validates config file structure and required fields
- Validates all repository paths exist and contain `.git` directory
- Handles git command failures with descriptive errors
- Database writes use transactions for atomicity
- Git log parsing continues on individual line parse errors
- Binary files (numstat showing `-	-`) are skipped

### Performance optimizations
- Transactions for bulk inserts
- Prepared statements for commits and file_changes
- Streaming line-by-line parsing (no loading full output into memory)
- In-memory aggregation for component contributions
- Single transaction per repository for commits/file_changes
- Single transaction for all component contributions

### Verbose output
When `-v` or `--verbose` is enabled:
- Shows output database path
- Logs each repository as it's processed
- Shows commit count per repository
- Shows first 5 pattern matches per component/repo (helps debug patterns)
- Shows total component contribution combinations computed

## Datasette Integration

### No direct integration needed
- Tool outputs standalone `.db` file
- User runs separately: `datasette report.db`
- Datasette provides:
  - Web-based query interface
  - JSON API
  - Export capabilities (CSV, JSON)
  - Plugin ecosystem for custom visualizations

### Example Datasette queries
```sql
-- Top contributors by component
SELECT 
  c.name as component,
  cc.author,
  cc.commit_count,
  cc.total_additions,
  cc.total_deletions
FROM component_contributions cc
JOIN components c ON cc.component_id = c.id
ORDER BY cc.commit_count DESC;

-- Cross-repository contributor activity
SELECT 
  r.name as repository,
  c.author,
  COUNT(*) as commits
FROM commits c
JOIN repositories r ON c.repository_id = r.id
GROUP BY r.name, c.author
ORDER BY commits DESC;

-- Component ownership analysis
SELECT 
  component,
  author,
  ROUND(100.0 * commit_count / SUM(commit_count) OVER (PARTITION BY component), 2) as percentage
FROM component_contributions cc
JOIN components c ON cc.component_id = c.id;
```

## Future Enhancements
- Datasette metadata.json generation for UI customization
- Branch comparison capabilities
- Merge commit handling options
- Component dependency visualization
- Time-series analysis support
