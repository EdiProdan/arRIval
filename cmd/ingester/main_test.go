package main

import (
	"testing"
	"time"
)

func TestLoadIngesterTimingConfigDefaults(t *testing.T) {
	t.Setenv("ARRIVAL_INGESTER_POLL_INTERVAL", "")
	t.Setenv("ARRIVAL_INGESTER_MAX_BACKOFF", "")

	pollInterval, maxBackoff := loadIngesterTimingConfig()
	if pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %s, want %s", pollInterval, defaultPollInterval)
	}
	if maxBackoff != defaultMaxBackoff {
		t.Fatalf("maxBackoff = %s, want %s", maxBackoff, defaultMaxBackoff)
	}
}

func TestLoadIngesterTimingConfigInvalidFallback(t *testing.T) {
	t.Setenv("ARRIVAL_INGESTER_POLL_INTERVAL", "nope")
	t.Setenv("ARRIVAL_INGESTER_MAX_BACKOFF", "invalid")

	pollInterval, maxBackoff := loadIngesterTimingConfig()
	if pollInterval != defaultPollInterval {
		t.Fatalf("pollInterval = %s, want %s", pollInterval, defaultPollInterval)
	}
	if maxBackoff != defaultMaxBackoff {
		t.Fatalf("maxBackoff = %s, want %s", maxBackoff, defaultMaxBackoff)
	}
}

func TestLoadIngesterTimingConfigClampsMaxBackoffToPollInterval(t *testing.T) {
	t.Setenv("ARRIVAL_INGESTER_POLL_INTERVAL", "5s")
	t.Setenv("ARRIVAL_INGESTER_MAX_BACKOFF", "3s")

	pollInterval, maxBackoff := loadIngesterTimingConfig()
	if pollInterval != 5*time.Second {
		t.Fatalf("pollInterval = %s, want %s", pollInterval, 5*time.Second)
	}
	if maxBackoff != 5*time.Second {
		t.Fatalf("maxBackoff = %s, want %s", maxBackoff, 5*time.Second)
	}
}

func TestNextPollDelaySuccessUsesBaseInterval(t *testing.T) {
	delay := nextPollDelay(5*time.Second, 2*time.Minute, 0, func() float64 { return 0.5 })
	if delay != 5*time.Second {
		t.Fatalf("delay = %s, want %s", delay, 5*time.Second)
	}
}

func TestNextPollDelayBackoffProgressionAndCap(t *testing.T) {
	base := 5 * time.Second
	maxBackoff := 2 * time.Minute

	cases := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 1, want: 5 * time.Second},
		{failures: 2, want: 10 * time.Second},
		{failures: 3, want: 20 * time.Second},
		{failures: 4, want: 40 * time.Second},
		{failures: 5, want: 80 * time.Second},
		{failures: 6, want: 120 * time.Second},
		{failures: 9, want: 120 * time.Second},
	}

	for _, tc := range cases {
		delay := nextPollDelay(base, maxBackoff, tc.failures, func() float64 { return 0.5 })
		if delay != tc.want {
			t.Fatalf("failures=%d delay=%s, want %s", tc.failures, delay, tc.want)
		}
	}
}

func TestNextPollDelayAppliesJitterWithinBounds(t *testing.T) {
	base := 5 * time.Second
	maxBackoff := 2 * time.Minute

	minDelay := nextPollDelay(base, maxBackoff, 1, func() float64 { return 0 })
	maxDelay := nextPollDelay(base, maxBackoff, 1, func() float64 { return 1 })

	if minDelay != 4*time.Second {
		t.Fatalf("minDelay = %s, want %s", minDelay, 4*time.Second)
	}
	if maxDelay != 6*time.Second {
		t.Fatalf("maxDelay = %s, want %s", maxDelay, 6*time.Second)
	}
}
