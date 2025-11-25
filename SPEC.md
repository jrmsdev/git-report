# git-report: Git Repository Contribution Report Tool

## Overview
A command-line tool written in Go that parses `git log` output from multiple repositories and generates a markdown report of project contributions.

## Purpose
Generate contributor reports by analyzing git history across one or more repositories, including commit metadata and file change patterns to track contributions to specific components of a project.

## Implementation Structure

### Code Organization
The implementation is split across multiple Go files for maintainability:

- **main.go**: CLI orchestration, flag parsing, main flow control
- **config.go**: Configuration loading, validation, and type definitions
- **database.go**: Database initialization, schema creation, data insertion
- **git.go**: Git command execution and log parsing
- **components.go**: Component contribution computation and path matching
- **report.go**: Markdown report generation [TO BE IMPLEMENTED]

### Working with this Specification
When modifying functionality:
1. Identify the relevant section(s) in this spec
2. Reference the "Implementation File" notation to know which Go file(s) to attach
3. Update both spec and implementation together to keep them synchronized

## Architecture

### Core Flow
**Implementation File: main.go**

1. Read configuration file specifying repositories and report parameters
2. Execute `git log` commands for each repository with structured output
3. Parse output into normalized data structures
4. Write parsed data to SQLite database (internal processing artifact)
5. Query database and generate markdown report
6. Output `.md` file with contribution analysis

### Technology Stack
- **Language**: Go (for performance and single-binary distribution)
- **Database**: SQLite3 (internal artifact for processing)
- **Output**: Markdown report file
- **Dependencies**: Keep them minimal

## Configuration File
**Implementation File: config.go**

### Format
YAML configuration file specifying repositories and report parameters.

### Example (YAML)
```yaml
output: project-report.md

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
Path to output markdown report file (default: `report.md`)

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
**Implementation File: database.go**

The SQLite database is an internal processing artifact (generated as `.report.db` in temp directory or alongside output file).

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
**Implementation File: git.go**

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
- Renames: `<additions><tab><deletions><tab>old/path => new/path` (extract new path from string containing ` => `, marks as 'R')
- Change type inference:
  - 'R': rename (detected by ` => ` in filepath string)
  - 'A': addition (additions > 0, deletions = 0)
  - 'D': deletion (additions = 0, deletions > 0)
  - 'M': modification (all other cases)

## CLI Interface
**Implementation File: main.go**

### Basic usage
```bash
git-report [config.yaml]
```

If no config file is specified, defaults to `report.yaml`.

### Optional flags
- `-config <path>`: path to configuration file
- `-verbose`: verbose output
- `-dry-run`: validate config without generating report

### Behavior
- Positional argument (if provided) is used as config file path, overriding `-config` flag
- Verbose mode shows: repository names as processed, commit counts per repository, and total component contribution count

## Output Files
**Implementation Files: main.go, report.go**

### Markdown report file
- Filename specified in config `output` field (default: `report.md`)
- If file exists, it is overwritten
- Contains formatted contribution analysis

### Database file (internal)
- Hidden/temporary artifact used for processing
- Not intended for direct user consumption
- May be cleaned up after report generation

## Markdown Report Format
**Implementation File: report.go [TO BE IMPLEMENTED]**

The generated markdown report includes:

1. **Report header**: Title, date range, repositories analyzed
2. **Overall statistics**: Total commits, contributors, lines changed
3. **Top contributors**: Ranked by commits across all repositories
4. **Repository breakdown**: Per-repository contribution statistics
5. **Component analysis** (if configured): Contributions by component
6. **Detailed contributor tables**: Per-author statistics with additions/deletions

Example structure:
```markdown
# Git Contribution Report

**Period**: 2024-01-01 to 2025-12-31  
**Repositories**: backend, frontend

## Summary

- Total commits: 1,234
- Contributors: 15
- Lines added: 45,678
- Lines deleted: 12,345

## Top Contributors

| Author | Commits | Additions | Deletions |
|--------|---------|-----------|-----------|
| alice@example.com | 456 | 23,456 | 5,678 |
| bob@example.com | 345 | 15,432 | 4,321 |

## Component Contributions

### API
| Author | Commits | Additions | Deletions |
|--------|---------|-----------|-----------|
...
```

## Component Analysis
**Implementation File: components.go**

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
**Across all files:**
- `os/exec`: execute git commands (git.go)
- `database/sql` + `github.com/mattn/go-sqlite3`: SQLite operations (database.go, git.go, components.go, report.go)
- `gopkg.in/yaml.v3`: YAML config parsing (config.go)
- `flag`: CLI argument parsing (main.go)
- `encoding/json`: JSON encoding for component path patterns (database.go)
- `bufio`: streaming line-by-line parsing (git.go)
- `path/filepath`: used in single-wildcard pattern matching (config.go, components.go)
- `strings`: string manipulation (all files)
- `time`: timestamp parsing (git.go)
- `os`: file operations (config.go, database.go, report.go)
- `fmt`: formatting output (report.go)

### Error handling
**Relevant files: main.go, config.go, git.go, database.go, components.go, report.go**
- Validates config file structure and required fields
- Validates all repository paths exist and contain `.git` directory
- Handles git command failures with descriptive errors
- Database writes use transactions for atomicity
- Git log parsing continues on individual line parse errors
- Binary files (numstat showing `-	-`) are skipped

### Performance optimizations
**Relevant files: git.go, components.go, database.go**
- Transactions for bulk inserts
- Prepared statements for commits and file_changes
- Streaming line-by-line parsing (no loading full output into memory)
- In-memory aggregation for component contributions
- Single transaction per repository for commits/file_changes
- Single transaction for all component contributions

### Verbose output
**Relevant files: main.go, git.go, components.go, report.go**

When `-verbose` is enabled:
- Log each repository name as it's processed
- Show commit count after processing each repository
- Show total component contribution count after computation
- Show report generation progress

## Future Enhancements
- Branch comparison capabilities
- Merge commit handling options
- Component dependency visualization
- Time-series analysis support
- Multiple output formats (HTML, JSON)
- Interactive charts/graphs in HTML output
