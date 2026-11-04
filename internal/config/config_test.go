package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestConfigEnvironmentSelectionOverridesAndValidation(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("APP_POLL_INTERVAL", "11s")
	c, err := LoadFrom("../../configs")
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != "development" || c.Poll.Interval != 11*time.Second || c.Kubernetes.RequestTimeout != 20*time.Second || c.Kubernetes.Attempts != 3 {
		t.Fatalf("bad file settings/override: %+v", c)
	}
	t.Setenv("APP_POLL_INTERVAL", "0s")
	if _, err := LoadFrom("../../configs"); err == nil {
		t.Fatal("accepted zero interval")
	}
}

func TestInlineInputsDecodedIntoConfig(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_USER", `{"data":[{"internals":{"opaque":{"username":"test-user","groups":["dev"]}}}]}`)
	t.Setenv("APP_POLICY", `{"namespaces":["default"],"permissions":[{"api_group":"","resource":"secrets","verbs":["get"]}]}`)
	c, err := LoadFrom("../../configs")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.User) != 1 || c.User[0].Username != "test-user" || c.Policy.Permissions[0].Resource != "secrets" {
		t.Fatalf("inputs not decoded: %+v", c)
	}
	t.Setenv("APP_USER", `{"data":[]} {}`)
	if _, err := LoadFrom("../../configs"); err == nil {
		t.Fatal("invalid inline inventory accepted")
	}
}

func TestConfigRequiresSelectedFileAndRequiredSettings(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	dir := t.TempDir()
	if _, err := LoadFrom(dir); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte("poll:\n  interval: 5m\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(dir); err == nil {
		t.Fatal("missing settings filled by code defaults")
	}
}

func TestEnvironmentOverrideForKeyAbsentFromYAML(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_KUBERNETES_ATTEMPTS", "4")
	b, err := os.ReadFile("../../configs/config.local.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	text := strings.Replace(string(b), "  attempts: 3\n", "", 1)
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Kubernetes.Attempts != 4 {
		t.Fatalf("attempts=%d want=4", c.Kubernetes.Attempts)
	}
}

func TestConfigSupportedEnvironments(t *testing.T) {
	for _, env := range []string{"local", "development", "production", ""} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			c, err := LoadFrom("../../configs")
			if err != nil {
				t.Fatal(err)
			}
			want := env
			if want == "" {
				want = "local"
			}
			if c.Env != want {
				t.Fatalf("env=%q want=%q", c.Env, want)
			}
		})
	}
	t.Setenv("APP_ENV", "staging")
	if _, err := LoadFrom("../../configs"); err == nil {
		t.Fatal("unsupported environment accepted")
	}
}

func TestInventorySizeLimit(t *testing.T) {
	p := writeInput(t, string(make([]byte, 8*1024*1024+1)))
	if _, err := LoadInventory(p); err == nil {
		t.Fatal("accepted oversized input")
	}
}
