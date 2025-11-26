// Copyright Jeremías Casteglione <jrmsdev@gmail.com>
// See LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/russross/blackfriday/v2"
)

func startHTTPServer(addr string, reportPath string, verbose bool) error {
	// Parse address, default to 127.0.0.1 if host not specified
	if !strings.Contains(addr, ":") {
		return fmt.Errorf("invalid address format, expected host:port or :port")
	}

	parts := strings.Split(addr, ":")
	if parts[0] == "" {
		addr = "127.0.0.1:" + parts[1]
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		content, err := os.ReadFile(reportPath)
		if err != nil {
			http.Error(w, "Report file not found", http.StatusNotFound)
			return
		}

		html := blackfriday.Run(content)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Write complete HTML document
		w.Write([]byte("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>Report</title>\n</head>\n<body>\n"))
		w.Write(html)
		w.Write([]byte("\n</body>\n</html>"))
	})

	if verbose {
		log.Printf("Starting HTTP server at http://%s", addr)
		log.Printf("Serving report: %s", reportPath)
	}

	fmt.Printf("Server running at http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}
