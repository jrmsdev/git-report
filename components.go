// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"database/sql"
	"log"
	"path/filepath"
	"strings"
)

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
