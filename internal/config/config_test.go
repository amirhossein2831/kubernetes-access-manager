package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeInput(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(p, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInventoryRejectsAmbiguity(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate_key":  `{"data":[{"internals":{"opaque":{"username":"a","username":"b","groups":[]}}}]}`,
		"duplicate_user": `{"data":[{"internals":{"one":{"username":"a","groups":[]},"two":{"username":"a","groups":[]}}}]}`,
		"blank":          `{"data":[{"internals":{"id":{"username":" ","groups":[]}}}]}`,
		"trailing":       `{"data":[]} {}`,
		"unknown":        `{"data":[],"typo":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadInventory(writeInput(t, input)); err == nil {
				t.Fatal("accepted invalid inventory")
			}
		})
	}
}

func TestInventoryOpaqueKeysAndSuppliedGroups(t *testing.T) {
	users, err := LoadInventory(writeInput(t, `{"metadata":{"version":"v1","timestamp":"2025-12-10T15:00:00Z"},"data":[{"configs":[],"internals":{"not-a-uuid":{"username":"ali","groups":["dev","dev"]}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != "ali" || len(users[0].Groups) != 1 || users[0].Groups[0] != "dev" {
		t.Fatalf("unexpected users: %+v", users)
	}
}

func TestConfigDefaultsOverridesAndValidation(t *testing.T) {
	t.Setenv("APP_POLL_INTERVAL", "11s")
	c, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.Poll.Interval != 11*time.Second || c.Kubernetes.RequestTimeout != 20*time.Second || c.Kubernetes.Attempts != 3 {
		t.Fatalf("bad defaults/override: %+v", c)
	}
	t.Setenv("APP_POLL_INTERVAL", "0s")
	if _, err := LoadFrom(t.TempDir()); err == nil {
		t.Fatal("accepted zero interval")
	}
}

func TestInventorySizeLimit(t *testing.T) {
	p := writeInput(t, string(make([]byte, 8*1024*1024+1)))
	if _, err := LoadInventory(p); err == nil {
		t.Fatal("accepted oversized input")
	}
}
