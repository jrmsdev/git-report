// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

type reportData struct {
	totalCommits      int
	totalContributors int
	totalAdditions    int64
	totalDeletions    int64
	repositories      []string
	topContributors   []contributorStats
	repoBreakdown     map[string]repoStats
	componentData     map[string][]contributorStats
}

type contributorStats struct {
	author    string
	email     string
	commits   int
	additions int64
	deletions int64
}

type repoStats struct {
	commits      int
	contributors int
	additions    int64
	deletions    int64
}

func generateReport(db *sql.DB, config *Config, outputPath string) error {
	data, err := collectReportData(db, config)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	writeReportHeader(f, config, data)
	writeOverallStatistics(f, data)
	writeTopContributors(f, data)
	writeRepositoryBreakdown(f, data)
	writeComponentAnalysis(f, data)

	return nil
}

func collectReportData(db *sql.DB, config *Config) (*reportData, error) {
	data := &reportData{
		repoBreakdown: make(map[string]repoStats),
		componentData: make(map[string][]contributorStats),
	}

	// Get repository names
	rows, err := db.Query("SELECT name FROM repositories ORDER BY name")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		data.repositories = append(data.repositories, name)
	}
	rows.Close()

	// Overall statistics
	err = db.QueryRow(`
		SELECT 
			COUNT(DISTINCT hash),
			COUNT(DISTINCT email),
			COALESCE(SUM(additions), 0),
			COALESCE(SUM(deletions), 0)
		FROM commits c
		LEFT JOIN file_changes fc ON c.hash = fc.commit_hash
	`).Scan(&data.totalCommits, &data.totalContributors, &data.totalAdditions, &data.totalDeletions)
	if err != nil {
		return nil, err
	}

	// Top contributors
	rows, err = db.Query(`
		SELECT 
			c.author,
			c.email,
			COUNT(DISTINCT c.hash) as commits,
			COALESCE(SUM(fc.additions), 0) as additions,
			COALESCE(SUM(fc.deletions), 0) as deletions
		FROM commits c
		LEFT JOIN file_changes fc ON c.hash = fc.commit_hash
		GROUP BY c.email
		ORDER BY commits DESC, additions DESC
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var stats contributorStats
		if err := rows.Scan(&stats.author, &stats.email, &stats.commits, &stats.additions, &stats.deletions); err != nil {
			rows.Close()
			return nil, err
		}
		data.topContributors = append(data.topContributors, stats)
	}
	rows.Close()

	// Repository breakdown
	for _, repoName := range data.repositories {
		var stats repoStats
		err = db.QueryRow(`
			SELECT 
				COUNT(DISTINCT c.hash),
				COUNT(DISTINCT c.email),
				COALESCE(SUM(fc.additions), 0),
				COALESCE(SUM(fc.deletions), 0)
			FROM commits c
			JOIN repositories r ON c.repository_id = r.id
			LEFT JOIN file_changes fc ON c.hash = fc.commit_hash
			WHERE r.name = ?
		`, repoName).Scan(&stats.commits, &stats.contributors, &stats.additions, &stats.deletions)
		if err != nil {
			return nil, err
		}
		data.repoBreakdown[repoName] = stats
	}

	// Component contributions
	if len(config.Components) > 0 {
		rows, err = db.Query(`
			SELECT 
				comp.name,
				cc.author,
				cc.email,
				cc.commit_count,
				cc.total_additions,
				cc.total_deletions
			FROM component_contributions cc
			JOIN components comp ON cc.component_id = comp.id
			ORDER BY comp.name, cc.commit_count DESC, cc.total_additions DESC
		`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var compName string
			var stats contributorStats
			if err := rows.Scan(&compName, &stats.author, &stats.email, &stats.commits, &stats.additions, &stats.deletions); err != nil {
				rows.Close()
				return nil, err
			}
			data.componentData[compName] = append(data.componentData[compName], stats)
		}
		rows.Close()
	}

	return data, nil
}

func writeReportHeader(f *os.File, config *Config, data *reportData) {
	fmt.Fprintln(f, "# Git Contribution Report")
	fmt.Fprintln(f)

	// Date range
	if config.Filters.Since != "" || config.Filters.Until != "" {
		fmt.Fprint(f, "**Period**: ")
		if config.Filters.Since != "" {
			fmt.Fprint(f, config.Filters.Since)
		} else {
			fmt.Fprint(f, "beginning")
		}
		fmt.Fprint(f, " to ")
		if config.Filters.Until != "" {
			fmt.Fprint(f, config.Filters.Until)
		} else {
			fmt.Fprint(f, time.Now().Format("2006-01-02"))
		}
		fmt.Fprintln(f)
	}

	// Repositories
	fmt.Fprintf(f, "**Repositories**: %s\n", strings.Join(data.repositories, ", "))
	fmt.Fprintln(f)
}

func writeOverallStatistics(f *os.File, data *reportData) {
	fmt.Fprintln(f, "## Summary")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "- Total commits: %s\n", formatNumber(data.totalCommits))
	fmt.Fprintf(f, "- Contributors: %d\n", data.totalContributors)
	fmt.Fprintf(f, "- Lines added: %s\n", formatNumber(int(data.totalAdditions)))
	fmt.Fprintf(f, "- Lines deleted: %s\n", formatNumber(int(data.totalDeletions)))
	fmt.Fprintln(f)
}

func writeTopContributors(f *os.File, data *reportData) {
	fmt.Fprintln(f, "## Top Contributors")
	fmt.Fprintln(f)
	fmt.Fprintln(f, "| Author | Commits | Additions | Deletions |")
	fmt.Fprintln(f, "|--------|---------|-----------|-----------|")

	for _, stats := range data.topContributors {
		fmt.Fprintf(f, "| %s | %s | %s | %s |\n",
			stats.email,
			formatNumber(stats.commits),
			formatNumber(int(stats.additions)),
			formatNumber(int(stats.deletions)))
	}
	fmt.Fprintln(f)
}

func writeRepositoryBreakdown(f *os.File, data *reportData) {
	fmt.Fprintln(f, "## Repository Breakdown")
	fmt.Fprintln(f)

	for _, repoName := range data.repositories {
		stats := data.repoBreakdown[repoName]
		fmt.Fprintf(f, "### %s\n", repoName)
		fmt.Fprintln(f)
		fmt.Fprintf(f, "- Commits: %s\n", formatNumber(stats.commits))
		fmt.Fprintf(f, "- Contributors: %d\n", stats.contributors)
		fmt.Fprintf(f, "- Lines added: %s\n", formatNumber(int(stats.additions)))
		fmt.Fprintf(f, "- Lines deleted: %s\n", formatNumber(int(stats.deletions)))
		fmt.Fprintln(f)
	}
}

func writeComponentAnalysis(f *os.File, data *reportData) {
	if len(data.componentData) == 0 {
		return
	}

	fmt.Fprintln(f, "## Component Contributions")
	fmt.Fprintln(f)

	for compName, contributors := range data.componentData {
		fmt.Fprintf(f, "### %s\n", compName)
		fmt.Fprintln(f)
		fmt.Fprintln(f, "| Author | Commits | Additions | Deletions |")
		fmt.Fprintln(f, "|--------|---------|-----------|-----------|")

		for _, stats := range contributors {
			fmt.Fprintf(f, "| %s | %s | %s | %s |\n",
				stats.email,
				formatNumber(stats.commits),
				formatNumber(int(stats.additions)),
				formatNumber(int(stats.deletions)))
		}
		fmt.Fprintln(f)
	}
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}
