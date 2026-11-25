package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, dsn, addr, origins string
		valid                    bool
	}{
		{"defaults", "postgres://u:p@localhost:5432/cms", "", "", true},
		{"explicit", "postgresql://localhost/cms?sslmode=require", "127.0.0.1:4101", " http://localhost:5173,https://example.com, ", true},
		{"ipv6", "postgres://[::1]/cms", "[::1]:4101", "", true},
		{"missing database", "", "", "", false},
		{"wrong scheme", "mysql://localhost/cms", "", "", false},
		{"missing host", "postgres:///cms", "", "", false},
		{"missing name", "postgres://localhost/", "", "", false},
		{"malformed", "postgres://%/cms", "", "", false},
		{"bad bind", "postgres://localhost/cms", "8080", "", false},
		{"zero port", "postgres://localhost/cms", ":0", "", false},
		{"large port", "postgres://localhost/cms", ":65536", "", false},
		{"named port", "postgres://localhost/cms", ":http", "", false},
		{"wildcard", "postgres://localhost/cms", "", "*", false},
		{"origin path", "postgres://localhost/cms", "", "https://example.com/", false},
		{"origin credentials", "postgres://localhost/cms", "", "https://u:p@example.com", false},
		{"origin query", "postgres://localhost/cms", "", "https://example.com?x=1", false},
		{"origin fragment", "postgres://localhost/cms", "", "https://example.com#x", false},
		{"origin scheme", "postgres://localhost/cms", "", "file://example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"DATABASE_URL": tc.dsn, "HTTP_ADDR": tc.addr, "CORS_ALLOWED_ORIGINS": tc.origins}
			cfg, err := Load(func(k string) string { return env[k] })
			if (err == nil) != tc.valid {
				t.Fatalf("config=%+v err=%v", cfg, err)
			}
			if tc.valid && cfg.Address == "" {
				t.Fatal("missing address")
			}
			if tc.name == "defaults" && cfg.Address != ":8080" {
				t.Fatalf("unexpected default: %s", cfg.Address)
			}
			if tc.name == "explicit" && len(cfg.AllowedOrigins) != 2 {
				t.Fatal("origins not normalized")
			}
		})
	}
}

func TestConfigErrorDoesNotExposePassword(t *testing.T) {
	_, err := Load(func(k string) string {
		if k == "DATABASE_URL" {
			return "bad://user:private-password@host/db"
		}
		return ""
	})
	if err == nil || strings.Contains(err.Error(), "private-password") {
		t.Fatalf("unsafe error: %v", err)
	}
}
