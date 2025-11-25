// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Output       string       `yaml:"output"`
	Repositories []Repository `yaml:"repositories"`
	Filters      Filters      `yaml:"filters"`
	Components   []Component  `yaml:"components"`
}

type Repository struct {
	Path string `yaml:"path"`
	Name string `yaml:"name"`
}

type Filters struct {
	Since   string   `yaml:"since"`
	Until   string   `yaml:"until"`
	Authors []string `yaml:"authors"`
	Branch  string   `yaml:"branch"`
}

type Component struct {
	Name  string   `yaml:"name"`
	Paths []string `yaml:"paths"`
}

type Commit struct {
	Hash         string
	RepositoryID int
	Author       string
	Email        string
	Date         time.Time
	Message      string
}

type FileChange struct {
	CommitHash string
	Filepath   string
	Additions  int
	Deletions  int
	ChangeType string
}

type DatasetteMeta struct {
	Title     string                  `json:"title,omitempty"`
	Databases map[string]DatabaseMeta `json:"databases"`
}

type DatabaseMeta struct {
	Tables map[string]TableMeta `json:"tables"`
}

type TableMeta struct {
	Description string            `json:"description,omitempty"`
	Hidden      bool              `json:"hidden,omitempty"`
	Label       string            `json:"label_column,omitempty"`
	Columns     map[string]string `json:"columns,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	verbose := flag.Bool("verbose", false, "verbose output")
	dryRun := flag.Bool("dry-run", false, "validate config without generating report")
	flag.Parse()

	// Positional argument overrides -config flag
	args := flag.Args()
	if len(args) > 0 {
		*configPath = args[0]
	}

	// Default to report.yaml if no config specified
	if *configPath == "" {
		*configPath = "report.yaml"
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := validateConfig(config); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	if *dryRun {
		fmt.Println("Configuration is valid")
		return
	}

	if config.Output == "" {
		config.Output = "report.db"
	}

	db, err := initDatabase(config.Output)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := createSchema(db); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	repoIDs := make(map[string]int)
	for _, repo := range config.Repositories {
		id, err := insertRepository(db, repo)
		if err != nil {
			log.Fatalf("Failed to insert repository %s: %v", repo.Name, err)
		}
		repoIDs[repo.Name] = id
	}

	if err := insertComponents(db, config.Components); err != nil {
		log.Fatalf("Failed to insert components: %v", err)
	}

	for _, repo := range config.Repositories {
		if *verbose {
			log.Printf("Processing repository: %s", repo.Name)
		}
		if err := processRepository(db, repo, repoIDs[repo.Name], config.Filters, *verbose); err != nil {
			log.Fatalf("Failed to process repository %s: %v", repo.Name, err)
		}
	}

	if err := computeComponentContributions(db, config.Components, config.Repositories, repoIDs, *verbose); err != nil {
		log.Fatalf("Failed to compute component contributions: %v", err)
	}

	// Generate Datasette metadata
	metaFilename := strings.TrimSuffix(config.Output, ".db") + "-metadata.json"
	if err := generateMetadata(config.Output, metaFilename); err != nil {
		log.Printf("Warning: failed to generate metadata: %v", err)
	} else if *verbose {
		log.Printf("Generated metadata file: %s", metaFilename)
	}

	if *verbose {
		log.Printf("Report generated successfully: %s", config.Output)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateConfig(config *Config) error {
	if len(config.Repositories) == 0 {
		return fmt.Errorf("no repositories specified")
	}

	for _, repo := range config.Repositories {
		if repo.Name == "" {
			return fmt.Errorf("repository name is required")
		}
		if repo.Path == "" {
			return fmt.Errorf("repository path is required")
		}
		if _, err := os.Stat(filepath.Join(repo.Path, ".git")); err != nil {
			return fmt.Errorf("invalid git repository: %s", repo.Path)
		}
	}

	return nil
}

func initDatabase(path string) (*sql.DB, error) {
	// Delete existing database file
	os.Remove(path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE repositories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		path TEXT NOT NULL
	);

	CREATE TABLE commits (
		hash TEXT PRIMARY KEY,
		repository_id INTEGER NOT NULL,
		author TEXT NOT NULL,
		email TEXT NOT NULL,
		date DATETIME NOT NULL,
		message TEXT NOT NULL,
		FOREIGN KEY (repository_id) REFERENCES repositories(id)
	);

	CREATE TABLE file_changes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		commit_hash TEXT NOT NULL,
		filepath TEXT NOT NULL,
		additions INTEGER NOT NULL,
		deletions INTEGER NOT NULL,
		change_type TEXT NOT NULL,
		FOREIGN KEY (commit_hash) REFERENCES commits(hash)
	);

	CREATE TABLE components (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		path_patterns TEXT NOT NULL
	);

	CREATE TABLE component_contributions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		component_id INTEGER NOT NULL,
		repository_id INTEGER NOT NULL,
		author TEXT NOT NULL,
		email TEXT NOT NULL,
		commit_count INTEGER NOT NULL,
		total_additions INTEGER NOT NULL,
		total_deletions INTEGER NOT NULL,
		FOREIGN KEY (component_id) REFERENCES components(id),
		FOREIGN KEY (repository_id) REFERENCES repositories(id)
	);

	CREATE INDEX idx_commits_repo ON commits(repository_id);
	CREATE INDEX idx_file_changes_commit ON file_changes(commit_hash);
	CREATE INDEX idx_component_contributions_component ON component_contributions(component_id);
	`

	_, err := db.Exec(schema)
	return err
}

func insertRepository(db *sql.DB, repo Repository) (int, error) {
	result, err := db.Exec("INSERT INTO repositories (name, path) VALUES (?, ?)", repo.Name, repo.Path)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}

func insertComponents(db *sql.DB, components []Component) error {
	for _, comp := range components {
		patterns, err := json.Marshal(comp.Paths)
		if err != nil {
			return err
		}
		_, err = db.Exec("INSERT INTO components (name, path_patterns) VALUES (?, ?)", comp.Name, string(patterns))
		if err != nil {
			return err
		}
	}
	return nil
}

func processRepository(db *sql.DB, repo Repository, repoID int, filters Filters, verbose bool) error {
	args := []string{"log", "--numstat", "--pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00"}

	if filters.Since != "" {
		args = append(args, fmt.Sprintf("--since=%s", filters.Since))
	}
	if filters.Until != "" {
		args = append(args, fmt.Sprintf("--until=%s", filters.Until))
	}
	for _, author := range filters.Authors {
		args = append(args, fmt.Sprintf("--author=%s", author))
	}
	if filters.Branch != "" {
		args = append(args, filters.Branch)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repo.Path

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git log failed: %v", err)
	}

	return parseGitLog(db, string(output), repoID, verbose)
}

func parseGitLog(db *sql.DB, output string, repoID int, verbose bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	commitStmt, err := tx.Prepare("INSERT INTO commits (hash, repository_id, author, email, date, message) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer commitStmt.Close()

	fileStmt, err := tx.Prepare("INSERT INTO file_changes (commit_hash, filepath, additions, deletions, change_type) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer fileStmt.Close()

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentCommit *Commit
	commitCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "\x00") {
			if currentCommit != nil {
				commitCount++
			}

			parts := strings.Split(line, "\x00")
			if len(parts) < 5 {
				continue
			}

			date, err := time.Parse("2006-01-02 15:04:05 -0700", parts[3])
			if err != nil {
				continue
			}

			currentCommit = &Commit{
				Hash:         parts[0],
				RepositoryID: repoID,
				Author:       parts[1],
				Email:        parts[2],
				Date:         date,
				Message:      parts[4],
			}

			_, err = commitStmt.Exec(currentCommit.Hash, currentCommit.RepositoryID,
				currentCommit.Author, currentCommit.Email, currentCommit.Date, currentCommit.Message)
			if err != nil {
				return err
			}
			continue
		}

		if currentCommit == nil || line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		adds, errAdds := strconv.Atoi(parts[0])
		dels, errDels := strconv.Atoi(parts[1])

		// Skip binary files (marked as "-" in numstat)
		if errAdds != nil || errDels != nil {
			continue
		}

		// Reconstruct the filepath (everything after the first two fields)
		filepath := strings.Join(parts[2:], " ")
		changeType := "M"

		// Handle renames: "old.txt => new.txt"
		if strings.Contains(filepath, " => ") {
			renameParts := strings.Split(filepath, " => ")
			if len(renameParts) == 2 {
				filepath = renameParts[1]
				changeType = "R"
			}
		} else {
			// Determine change type from the stats
			if adds > 0 && dels == 0 {
				changeType = "A"
			} else if adds == 0 && dels > 0 {
				changeType = "D"
			}
		}

		_, err := fileStmt.Exec(currentCommit.Hash, filepath, adds, dels, changeType)
		if err != nil {
			return err
		}
	}

	if verbose && commitCount > 0 {
		log.Printf("  Processed %d commits", commitCount)
	}

	return tx.Commit()
}

func computeComponentContributions(db *sql.DB, components []Component, repos []Repository, repoIDs map[string]int, verbose bool) error {
	type contribKey struct {
		componentID  int
		repositoryID int
		email        string
	}

	contributions := make(map[contribKey]struct {
		author    string
		commits   map[string]bool
		additions int
		deletions int
	})

	for _, comp := range components {
		var componentID int
		err := db.QueryRow("SELECT id FROM components WHERE name = ?", comp.Name).Scan(&componentID)
		if err != nil {
			return err
		}

		patterns := make(map[string][]string)
		for _, pattern := range comp.Paths {
			parts := strings.SplitN(pattern, ":", 2)
			if len(parts) != 2 {
				continue
			}
			repoName := parts[0]
			pathPattern := parts[1]
			patterns[repoName] = append(patterns[repoName], pathPattern)
		}

		for repoName, repoPatterns := range patterns {
			repoID, ok := repoIDs[repoName]
			if !ok {
				continue
			}

			rows, err := db.Query(`
				SELECT c.hash, c.author, c.email, fc.additions, fc.deletions, fc.filepath
				FROM commits c
				JOIN file_changes fc ON c.hash = fc.commit_hash
				WHERE c.repository_id = ?
			`, repoID)
			if err != nil {
				return err
			}

			for rows.Next() {
				var hash, author, email, filepath string
				var additions, deletions int
				if err := rows.Scan(&hash, &author, &email, &additions, &deletions, &filepath); err != nil {
					rows.Close()
					return err
				}

				matched := false
				for _, pattern := range repoPatterns {
					if matchPath(filepath, pattern) {
						matched = true
						break
					}
				}

				if matched {
					key := contribKey{componentID, repoID, email}
					contrib := contributions[key]
					contrib.author = author
					if contrib.commits == nil {
						contrib.commits = make(map[string]bool)
					}
					contrib.commits[hash] = true
					contrib.additions += additions
					contrib.deletions += deletions
					contributions[key] = contrib
				}
			}
			rows.Close()
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO component_contributions 
		(component_id, repository_id, author, email, commit_count, total_additions, total_deletions)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for key, contrib := range contributions {
		_, err := stmt.Exec(key.componentID, key.repositoryID, contrib.author, key.email,
			len(contrib.commits), contrib.additions, contrib.deletions)
		if err != nil {
			return err
		}
	}

	if verbose {
		log.Printf("Computed %d component contributions", len(contributions))
	}

	return tx.Commit()
}

func matchPath(path, pattern string) bool {
	// Exact match
	if path == pattern {
		return true
	}

	// Handle ** (match any number of directories)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")

		// Pattern: **/something
		if len(parts) == 2 && parts[0] == "" {
			suffix := strings.TrimPrefix(parts[1], "/")
			if suffix == "" {
				return true // Just "**" matches everything
			}
			return strings.HasSuffix(path, suffix) || strings.Contains(path, "/"+suffix)
		}

		// Pattern: something/**
		if len(parts) == 2 && parts[1] == "" {
			prefix := strings.TrimSuffix(parts[0], "/")
			return strings.HasPrefix(path, prefix+"/") || path == prefix
		}

		// Pattern: prefix/**/suffix
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}
			if suffix != "" && !strings.HasSuffix(path, suffix) {
				return false
			}
			return true
		}
	}

	// Handle single * (match within a single directory level)
	if strings.Contains(pattern, "*") && !strings.Contains(pattern, "**") {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	return false
}

func generateMetadata(dbPath string, outputPath string) error {
	dbName := filepath.Base(dbPath)

	meta := DatasetteMeta{
		Title: "Git Contribution Report",
		Databases: map[string]DatabaseMeta{
			dbName: {
				Tables: map[string]TableMeta{
					"repositories": {
						Description: "Repositories analyzed in this report",
						Label:       "name",
					},
					"commits": {
						Description: "Individual commits from all repositories",
					},
					"file_changes": {
						Description: "File-level changes per commit",
						Hidden:      true,
					},
					"components": {
						Description: "Project components defined in config",
						Label:       "name",
					},
					"component_contributions": {
						Description: "Aggregated contributor statistics by component",
						Columns: map[string]string{
							"commit_count":    "Number of commits",
							"total_additions": "Lines added",
							"total_deletions": "Lines removed",
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}
