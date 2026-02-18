package envutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDotEnvIgnoresCommentsAndDoesNotOverride(t *testing.T) {
	t.Setenv("ARRIVAL_TEST_ENV_KEEP", "from-env")
	oldNew, hadNew := os.LookupEnv("ARRIVAL_TEST_ENV_NEW")
	if err := os.Unsetenv("ARRIVAL_TEST_ENV_NEW"); err != nil {
		t.Fatalf("Unsetenv ARRIVAL_TEST_ENV_NEW: %v", err)
	}
	t.Cleanup(func() {
		if hadNew {
			_ = os.Setenv("ARRIVAL_TEST_ENV_NEW", oldNew)
			return
		}
		_ = os.Unsetenv("ARRIVAL_TEST_ENV_NEW")
	})

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `
# comment
ARRIVAL_TEST_ENV_KEEP=from-file
ARRIVAL_TEST_ENV_NEW='new-value'
INVALID_LINE
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp .env: %v", err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}

	if got := os.Getenv("ARRIVAL_TEST_ENV_KEEP"); got != "from-env" {
		t.Fatalf("ARRIVAL_TEST_ENV_KEEP = %q, want %q", got, "from-env")
	}
	if got := os.Getenv("ARRIVAL_TEST_ENV_NEW"); got != "new-value" {
		t.Fatalf("ARRIVAL_TEST_ENV_NEW = %q, want %q", got, "new-value")
	}
}

func TestCSVEnvTrimsAndFallsBack(t *testing.T) {
	t.Setenv("ARRIVAL_TEST_CSV", " a , , b ,, c ")
	got := CSVEnv("ARRIVAL_TEST_CSV", "x,y")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("CSVEnv len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CSVEnv[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	t.Setenv("ARRIVAL_TEST_CSV", ",,,")
	got = CSVEnv("ARRIVAL_TEST_CSV", "fallback-a,fallback-b")
	want = []string{"fallback-a", "fallback-b"}
	if len(got) != len(want) {
		t.Fatalf("CSVEnv fallback len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CSVEnv fallback[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDurationEnvParsesAndFlagsInvalid(t *testing.T) {
	t.Setenv("ARRIVAL_TEST_DURATION", "45")
	d, invalid := DurationEnv("ARRIVAL_TEST_DURATION", 5*time.Second)
	if invalid {
		t.Fatalf("invalid = true, want false")
	}
	if d != 45*time.Second {
		t.Fatalf("duration = %s, want %s", d, 45*time.Second)
	}

	t.Setenv("ARRIVAL_TEST_DURATION", "2m30s")
	d, invalid = DurationEnv("ARRIVAL_TEST_DURATION", 5*time.Second)
	if invalid {
		t.Fatalf("invalid = true, want false")
	}
	if d != 150*time.Second {
		t.Fatalf("duration = %s, want %s", d, 150*time.Second)
	}

	t.Setenv("ARRIVAL_TEST_DURATION", "invalid")
	d, invalid = DurationEnv("ARRIVAL_TEST_DURATION", 5*time.Second)
	if !invalid {
		t.Fatalf("invalid = false, want true")
	}
	if d != 5*time.Second {
		t.Fatalf("duration = %s, want %s", d, 5*time.Second)
	}
}
