package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
)

type datasetSource struct {
	name string
	url  string
	file string
}

var sources = []datasetSource{
	{name: "linije", url: "http://e-usluge2.rijeka.hr/OpenData/ATlinije.json", file: "linije.json"},
	{name: "stanice", url: "http://e-usluge2.rijeka.hr/OpenData/ATstanice.json", file: "stanice.json"},
	{name: "voznired_dnevni", url: "http://e-usluge2.rijeka.hr/OpenData/ATvoznired.json", file: "voznired_dnevni.json"},
}

func main() {
	dataDir := flag.String("data-dir", "data", "directory for output static json files")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	client := &http.Client{Timeout: defaultTimeout}

	for _, source := range sources {
		if err := fetchAndSave(client, source, *dataDir); err != nil {
			log.Fatalf("sync %s: %v", source.name, err)
		}
	}

	fmt.Printf("synced %d static datasets into %s\n", len(sources), *dataDir)
}

func fetchAndSave(client *http.Client, source datasetSource, dataDir string) error {
	resp, err := client.Get(source.url)
	if err != nil {
		return fmt.Errorf("request %s: %w", source.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("request %s returned status %d", source.url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s body: %w", source.url, err)
	}

	outputPath := filepath.Join(dataDir, source.file)
	if err := os.WriteFile(outputPath, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}

	return nil
}
