package mysql

import (
	"strings"
	"testing"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestBuildDSNAppendsDefaultPort(t *testing.T) {
	dsn, err := BuildDSN(envMap(map[string]string{
		EnvHost:     "db",
		EnvUsername: "conspectus",
		EnvPassword: "s3cret",
		EnvDatabase: "asset_tracker",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "tcp(db:3306)/asset_tracker") {
		t.Fatalf("dsn %q missing host:port/db", dsn)
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Loc != time.UTC {
		t.Fatalf("loc = %v, want UTC", cfg.Loc)
	}
	if !cfg.ParseTime {
		t.Fatal("parseTime must be set")
	}
}

func TestBuildDSNKeepsExplicitPort(t *testing.T) {
	dsn, err := BuildDSN(envMap(map[string]string{
		EnvHost:     "db:13306",
		EnvDatabase: "asset_tracker",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "tcp(db:13306)/") {
		t.Fatalf("dsn %q must keep the explicit port", dsn)
	}
}

func TestBuildDSNRequiresHostAndDatabase(t *testing.T) {
	_, err := BuildDSN(envMap(nil))
	if err == nil || !strings.Contains(err.Error(), EnvHost) {
		t.Fatalf("want %s required error, got %v", EnvHost, err)
	}
	_, err = BuildDSN(envMap(map[string]string{EnvHost: "db"}))
	if err == nil || !strings.Contains(err.Error(), EnvDatabase) {
		t.Fatalf("want %s required error, got %v", EnvDatabase, err)
	}
}

func TestBuildDSNEscapesCredentials(t *testing.T) {
	dsn, err := BuildDSN(envMap(map[string]string{
		EnvHost:     "db",
		EnvDatabase: "x",
		EnvUsername: "us@er",
		EnvPassword: "p/a:ss?word",
	}))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("built DSN does not parse: %v", err)
	}
	if cfg.User != "us@er" || cfg.Passwd != "p/a:ss?word" {
		t.Fatalf("credentials round-trip failed: user=%q pass=%q", cfg.User, cfg.Passwd)
	}
	if cfg.Addr != "db:3306" || cfg.DBName != "x" || !cfg.ParseTime {
		t.Fatalf("unexpected parsed DSN: %+v", cfg)
	}
}
