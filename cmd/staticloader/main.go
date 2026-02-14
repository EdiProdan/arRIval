package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/EdiProdan/arRIval/internal/staticdata"
)

func main() {
	dataDir := flag.String("data-dir", "data", "path to static data directory")
	flag.Parse()

	store, err := staticdata.LoadFromDir(*dataDir)
	if err != nil {
		log.Fatalf("load static data: %v", err)
	}

	fmt.Printf("%d stations, %d line variants, %d departures loaded\n",
		len(store.Stations),
		len(store.LinePathsByLinVar),
		len(store.TimetableByPolazak),
	)
}
