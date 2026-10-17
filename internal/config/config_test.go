package config

import (
	"net/url"
	"strings"
	"testing"
)

func TestLoadMissingVariables(t *testing.T) {
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("POSTGRES_DB", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when required variables are missing")
	}
	for _, name := range []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q should mention %s", err, name)
		}
	}
}

func TestLoadBuildsDatabaseURL(t *testing.T) {
	t.Setenv("POSTGRES_USER", "app")
	t.Setenv("POSTGRES_PASSWORD", "p@ss:w/rd#1") // special chars to test escaping
	t.Setenv("POSTGRES_DB", "problemlog")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}

	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("DatabaseURL is not a valid URL: %v", err)
	}
	if got, _ := u.User.Password(); got != "p@ss:w/rd#1" {
		t.Errorf("password did not survive escaping, got %q", got)
	}
	if u.Host != "db:5432" || u.Path != "/problemlog" {
		t.Errorf("unexpected host/path: %s %s", u.Host, u.Path)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("POSTGRES_USER", "app")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "problemlog")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default Port = %q, want 8080", cfg.Port)
	}
	if !strings.Contains(cfg.DatabaseURL, "localhost:5432") {
		t.Errorf("default host/port missing in %q", cfg.DatabaseURL)
	}
}
