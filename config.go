// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"

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
