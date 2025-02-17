package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/isaacphi/slop/internal/config"
	"io"
	"os"
	"path/filepath"
)

func main() {
	var outFile string
	flag.StringVar(&outFile, "out", "schema.json", "Output file path")
	flag.Parse()

	// Convert to absolute path if relative
	if !filepath.IsAbs(outFile) {
		// Get the directory where the tool is being run from
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
			os.Exit(1)
		}
		outFile = filepath.Join(wd, outFile)
	}

	schema, err := config.GenerateJSONSchema()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schema: %v\n", err)
		os.Exit(1)
	}

	json, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
		os.Exit(1)
	}

	// Ensure the directory exists
	dir := filepath.Dir(outFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
		os.Exit(1)
	}

	// Write to the original location
	if err := os.WriteFile(outFile, json, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing schema to %s: %v\n", outFile, err)
		os.Exit(1)
	}
	fmt.Printf("Schema written to %s\n", outFile)

	// Copy to .slop directory
	slopDir := filepath.Join("..", "..", ".slop")
	if err := os.MkdirAll(slopDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .slop directory: %v\n", err)
		os.Exit(1)
	}

	destFile := filepath.Join(slopDir, filepath.Base(outFile))
	source, err := os.Open(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening source file: %v\n", err)
		os.Exit(1)
	}
	defer source.Close()

	destination, err := os.Create(destFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating destination file: %v\n", err)
		os.Exit(1)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying file to .slop directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Schema copied to %s\n", destFile)
}
