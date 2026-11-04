package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestInlineInputsRemainStrings(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_USER", `{"data":[{"internals":{"opaque":{"username":"test-user","groups":["dev"]}}}]}`)
	t.Setenv("APP_POLICY", `{"namespaces":["default"],"permissions":[{"api_group":"","resource":"secrets","verbs":["get"]}]}`)
	c, err := LoadFrom("../../configs")
	if err != nil {
		t.Fatal(err)
	}
	if c.User != os.Getenv("APP_USER") || c.Policy != os.Getenv("APP_POLICY") {
		t.Fatal("inline input strings changed")
	}
	t.Setenv("APP_USER", `{"data":[]} {}`)
	c, err = LoadFrom("../../configs")
	if err != nil || c.User != `{"data":[]} {}` {
		t.Fatal("config should leave JSON validation to the domain")
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
