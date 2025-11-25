// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

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
