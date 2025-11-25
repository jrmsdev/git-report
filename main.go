// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

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
