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

func main() {
	configPath := flag.String("c", "report.yaml", "path to configuration file")
	configFlag := flag.String("config", "", "path to configuration file")
	verbose := flag.Bool("v", false, "verbose output")
	verboseFlag := flag.Bool("verbose", false, "verbose output")
	dryRun := flag.Bool("dry-run", false, "validate config without generating report")
	flag.Parse()

	if *configFlag != "" {
		configPath = configFlag
	}
	isVerbose := *verbose || *verboseFlag

	args := flag.Args()
	if len(args) > 0 {
		configPath = &args[0]
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

	if isVerbose {
		log.Printf("Generating report: %s", config.Output)
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
		if isVerbose {
			log.Printf("Processing repository: %s", repo.Name)
		}
	}

	if err := insertComponents(db, config.Components); err != nil {
		log.Fatalf("Failed to insert components: %v", err)
	}

	for _, repo := range config.Repositories {
		if err := processRepository(db, repo, repoIDs[repo.Name], config.Filters, isVerbose); err != nil {
			log.Fatalf("Failed to process repository %s: %v", repo.Name, err)
		}
	}

	if err := computeComponentContributions(db, config.Components, config.Repositories, repoIDs); err != nil {
		log.Fatalf("Failed to compute component contributions: %v", err)
	}

	if isVerbose {
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
	args := []string{"log", "--numstat", "--name-status", "--pretty=format:%H%x00%an%x00%ae%x00%ai%x00%s%x00"}

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
		if len(parts) < 2 {
			continue
		}

		if len(parts) == 3 {
			adds, _ := strconv.Atoi(parts[0])
			dels, _ := strconv.Atoi(parts[1])
			filepath := parts[2]

			changeType := "M"
			_, err := fileStmt.Exec(currentCommit.Hash, filepath, adds, dels, changeType)
			if err != nil {
				return err
			}
		} else if len(parts) == 2 {
			changeType := parts[0]
			filepath := parts[1]
			_, err := fileStmt.Exec(currentCommit.Hash, filepath, 0, 0, changeType)
			if err != nil {
				return err
			}
		}
	}

	if verbose && commitCount > 0 {
		log.Printf("Processed %d commits", commitCount)
	}

	return tx.Commit()
}

func computeComponentContributions(db *sql.DB, components []Component, repos []Repository, repoIDs map[string]int) error {
	type contribKey struct {
		componentID  int
		repositoryID int
		email        string
	}

	contributions := make(map[contribKey]struct {
		author    string
		commits   int
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
					contrib.commits++
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
			contrib.commits, contrib.additions, contrib.deletions)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func matchPath(path, pattern string) bool {
	pattern = strings.ReplaceAll(pattern, "**", "§§")
	pattern = strings.ReplaceAll(pattern, "*", "[^/]*")
	pattern = strings.ReplaceAll(pattern, "§§", ".*")
	pattern = "^" + pattern + "$"

	matched := false
	for i := 0; i < len(path) && i < len(pattern); {
		if i < len(pattern)-1 && pattern[i] == '.' && pattern[i+1] == '*' {
			if i == len(pattern)-2 {
				matched = true
				break
			}
			matched = false
			for j := i; j <= len(path); j++ {
				if matchPath(path[j:], pattern[i+2:]) {
					matched = true
					break
				}
			}
			break
		} else if i < len(pattern)-6 && pattern[i:i+5] == "[^/]*" {
			j := i
			for j < len(path) && path[j] != '/' {
				j++
			}
			if matchPath(path[j:], pattern[i+5:]) {
				matched = true
			}
			break
		} else if i < len(path) && i < len(pattern) && path[i] == pattern[i] {
			i++
			if i == len(path) && i == len(pattern) {
				matched = true
			}
		} else {
			break
		}
	}

	if len(path) == 0 && len(pattern) == 0 {
		matched = true
	}

	return matched || simpleMatch(path, pattern)
}

func simpleMatch(path, pattern string) bool {
	match, _ := filepath.Match(pattern, path)
	if match {
		return true
	}

	parts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if strings.Contains(pattern, "**") {
		for i := range parts {
			if parts[i] == "**" {
				before := strings.Join(parts[:i], "/")
				after := strings.Join(parts[i+1:], "/")

				if before != "" {
					beforeMatch, _ := filepath.Match(before, strings.Join(pathParts[:min(i, len(pathParts))], "/"))
					if !beforeMatch {
						return false
					}
				}

				if after != "" {
					for j := i; j <= len(pathParts); j++ {
						afterPath := strings.Join(pathParts[j:], "/")
						afterMatch, _ := filepath.Match(after, afterPath)
						if afterMatch {
							return true
						}
					}
				} else {
					return true
				}
			}
		}
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
