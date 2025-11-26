// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	configPath := flag.String("config", "", "path to configuration file")
	verbose := flag.Bool("verbose", false, "verbose output")
	dryRun := flag.Bool("dry-run", false, "validate config without generating report")
	httpAddr := flag.String("http", "", "start HTTP server at specified address (e.g., :8045 or 127.0.0.1:8045)")
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
		config.Output = "report.md"
	}

	// If HTTP server mode is requested, start server and exit
	if *httpAddr != "" {
		if err := startHTTPServer(*httpAddr, config.Output, *verbose); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
		return
	}

	// Use a hidden database file for internal processing
	dbPath := filepath.Join(filepath.Dir(config.Output), "."+filepath.Base(config.Output)+".db")
	db, err := initDatabase(dbPath)
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

	// Generate markdown report
	if *verbose {
		log.Printf("Generating report: %s", config.Output)
	}
	if err := generateReport(db, config, config.Output); err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	// Clean up database file
	os.Remove(dbPath)

	if *verbose {
		log.Printf("Report generated successfully: %s", config.Output)
	}
}
