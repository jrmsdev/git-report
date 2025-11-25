// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

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
