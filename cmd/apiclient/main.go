package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/envutil"
)

const (
	defaultAPIBaseURL  = "https://www.rijekaplus.hr"
	defaultHTTPTimeout = 15 * time.Second
	defaultTokenTTL    = 60 * time.Minute
	defaultRefreshLead = 5 * time.Minute
)

func main() {
	if err := envutil.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	apiBaseURL := envutil.StringEnv("ARRIVAL_API_BASE_URL", defaultAPIBaseURL)
	username := envutil.FirstNonEmpty(
		os.Getenv("ARRIVAL_API_USER"),
		os.Getenv("ARRIVAL_API_USERNAME"),
	)
	password := envutil.FirstNonEmpty(
		os.Getenv("ARRIVAL_API_PASS"),
		os.Getenv("ARRIVAL_API_PASSWORD"),
	)

	if strings.TrimSpace(username) == "" {
		log.Fatalf("missing API username: set ARRIVAL_API_USERNAME (or ARRIVAL_API_USER)")
	}
	if strings.TrimSpace(password) == "" {
		log.Fatalf("missing API password: set ARRIVAL_API_PASSWORD (or ARRIVAL_API_PASS)")
	}

	httpTimeout, invalidTimeout := envutil.DurationEnv("ARRIVAL_API_TIMEOUT", defaultHTTPTimeout)
	if invalidTimeout {
		log.Printf("invalid duration for ARRIVAL_API_TIMEOUT=%q, using fallback %s", strings.TrimSpace(os.Getenv("ARRIVAL_API_TIMEOUT")), defaultHTTPTimeout)
	}
	tokenTTL, invalidTokenTTL := envutil.DurationEnv("ARRIVAL_API_TOKEN_TTL", defaultTokenTTL)
	if invalidTokenTTL {
		log.Printf("invalid duration for ARRIVAL_API_TOKEN_TTL=%q, using fallback %s", strings.TrimSpace(os.Getenv("ARRIVAL_API_TOKEN_TTL")), defaultTokenTTL)
	}
	refreshLead, invalidRefreshLead := envutil.DurationEnv("ARRIVAL_API_REFRESH_MARGIN", defaultRefreshLead)
	if invalidRefreshLead {
		log.Printf("invalid duration for ARRIVAL_API_REFRESH_MARGIN=%q, using fallback %s", strings.TrimSpace(os.Getenv("ARRIVAL_API_REFRESH_MARGIN")), defaultRefreshLead)
	}

	client, err := autotrolej.NewClient(autotrolej.Config{
		BaseURL:       apiBaseURL,
		Username:      username,
		Password:      password,
		Timeout:       httpTimeout,
		TokenTTL:      tokenTTL,
		RefreshMargin: refreshLead,
	})
	if err != nil {
		log.Fatalf("create API client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := client.GetAutobusi(ctx)
	if err != nil {
		log.Fatalf("fetch /autobusi: %v", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(response); err != nil {
		log.Fatalf("write stdout JSON: %v", err)
	}
}
