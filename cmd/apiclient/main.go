package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
)

const (
	defaultAPIBaseURL  = "https://www.rijekaplus.hr"
	defaultHTTPTimeout = 15 * time.Second
	defaultTokenTTL    = 60 * time.Minute
	defaultRefreshLead = 5 * time.Minute
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	apiBaseURL := getenv("ARRIVAL_API_BASE_URL", defaultAPIBaseURL)
	username := firstNonEmpty(
		os.Getenv("ARRIVAL_API_USER"),
		os.Getenv("ARRIVAL_API_USERNAME"),
	)
	password := firstNonEmpty(
		os.Getenv("ARRIVAL_API_PASS"),
		os.Getenv("ARRIVAL_API_PASSWORD"),
	)

	if strings.TrimSpace(username) == "" {
		log.Fatalf("missing API username: set ARRIVAL_API_USERNAME (or ARRIVAL_API_USER)")
	}
	if strings.TrimSpace(password) == "" {
		log.Fatalf("missing API password: set ARRIVAL_API_PASSWORD (or ARRIVAL_API_PASS)")
	}

	httpTimeout := getDurationEnv("ARRIVAL_API_TIMEOUT", defaultHTTPTimeout)
	tokenTTL := getDurationEnv("ARRIVAL_API_TOKEN_TTL", defaultTokenTTL)
	refreshLead := getDurationEnv("ARRIVAL_API_REFRESH_MARGIN", defaultRefreshLead)

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

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		keyValue := strings.SplitN(line, "=", 2)
		if len(keyValue) != 2 {
			continue
		}

		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		value = strings.Trim(value, `"'`)
		if key == "" || value == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s from %s: %w", key, path, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}

	return nil
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		log.Printf("invalid duration for %s=%q, using fallback %s", key, raw, fallback)
		return fallback
	}

	return duration
}
