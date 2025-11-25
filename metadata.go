// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
