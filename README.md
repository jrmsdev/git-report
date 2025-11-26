# git-report: Git Repository Contribution Report Tool

## Overview
A command-line tool written in Go that parses `git log` output from multiple repositories and generates a markdown report of project contributions.

## Purpose
Generate contributor reports by analyzing git history across one or more repositories, including commit metadata and file change patterns to track contributions to specific components of a project.

## CLI Interface

### Basic usage
```bash
git-report [config.yaml]
```

If no config file is specified, defaults to `report.yaml`.

### Optional flags
- `-config <path>`: path to configuration file
- `-verbose`: verbose output
- `-dry-run`: validate config without generating report
- `-http <address>`: start HTTP server at specified address (e.g., `:8045` or `127.0.0.1:8045`)

## Configuration File

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
