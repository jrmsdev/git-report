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
- **Dependencies**: Minimal external dependencies

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
- `id` (INTEGER, PRIMARY KEY)
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
- `id` (INTEGER, PRIMARY KEY)
- `commit_hash` (TEXT, FOREIGN KEY): references commits(hash)
- `filepath` (TEXT): path to changed file
- `additions` (INTEGER): lines added
- `deletions` (INTEGER): lines deleted
- `change_type` (TEXT): 'A' (added), 'M' (modified), 'D' (deleted), 'R' (renamed)

### `components` table
- `id` (INTEGER, PRIMARY KEY)
- `name` (TEXT, UNIQUE): component name from config
- `path_patterns` (TEXT): JSON array of path patterns

### `component_contributions` table
Pre-computed statistics for efficient querying:
- `id` (INTEGER, PRIMARY KEY)
- `component_id` (INTEGER, FOREIGN KEY): references components(id)
- `repository_id` (INTEGER, FOREIGN KEY): references repositories(id)
- `author` (TEXT)
- `email` (TEXT)
- `commit_count` (INTEGER)
- `total_additions` (INTEGER)
- `total_deletions` (INTEGER)

## Git Log Integration

### Required git log flags
- `--numstat`: get per-file addition/deletion statistics
- `--name-status`: get file operation types (A/M/D/R)
- `--pretty=format:...`: structured commit metadata output
- Filters from config: `--since`, `--until`, `--author`, `--branch`

### Git log format
```
--pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00 --numstat --name-status
```
(Using null bytes `%x00` as field delimiters for reliable parsing)

## CLI Interface

### Basic usage
```bash
git-report config.yaml
```

### Optional flags
- `-c, --config`: path to configuration file (default: `report.yaml`)
- `-v, --verbose`: verbose output
- `--dry-run`: validate config without generating report

## Component Analysis

### At parse time
1. Load component definitions from config
2. Parse all commits and file changes
3. Match file paths against component patterns for each repository
4. Compute aggregated statistics per component
5. Store in `component_contributions` table

### Path pattern matching
- Support glob patterns: `**` (recursive directories), `*` (wildcard)
- Implement custom glob matching using standard library (filepath.Match for simple patterns)
- Match against repository-prefixed paths
- Handle multiple patterns per component

## Implementation Notes

### Go packages needed
- `os/exec`: execute git commands
- `database/sql` + `github.com/mattn/go-sqlite3`: SQLite operations
- `gopkg.in/yaml.v3`: YAML config parsing
- Standard library for path matching (custom glob implementation or filepath.Match)
- `flag`: CLI argument parsing

### Error handling considerations
- Validate config file structure and required fields
- Validate all repository paths exist and are git repositories
- Handle git command failures gracefully
- Ensure database writes are atomic/transactional
- Validate parsed data before insertion

### Performance optimizations
- Use transactions for bulk inserts
- Prepared statements for repeated inserts
- Stream processing for large repos (don't load all data into memory)
- Parallel processing of multiple repositories

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
