package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultListen         = ":8080"
	DefaultDriver         = "mysql"
	DefaultTimezone       = "UTC"
	DefaultWebDir         = "/app/web"
	DefaultMigrationsDir  = "/app/migrations"
	DefaultLogLevel       = "info"
	DefaultLogFormat      = "text"
	WebDirFallback        = "./web"
	MigrationsDirFallback = "migrations"
)

type Config struct {
	Driver       string
	Timezone     *time.Location
	TimezoneName string

	WebDir       string
	MigrationDir string

	AutoMigrate bool
	ReadOnly    bool
	WebUI       bool

	APIURL string

	LogLevel   string
	LogFormat  string
	LogColours bool
}

type EnvFn func(string) string

func osEnv(key string) string { return os.Getenv(key) }

func Load() (Config, error) { return LoadEnv(osEnv) }

func LoadEnv(env EnvFn) (Config, error) {
	// The standard TZ variable picks the calendar timezone (month
	// bucketing, date display); UTC keeps bucketing stable when unset.
	tzName := env("TZ")
	if tzName == "" {
		tzName = DefaultTimezone
	}
	c := Config{
		Driver:       get(env, "DB_DRIVER", DefaultDriver),
		TimezoneName: tzName,
		WebDir:       get(env, "WEB_DIR", DefaultWebDir),
		MigrationDir: get(env, "MIGRATIONS_DIR", DefaultMigrationsDir),
		AutoMigrate:  boolEnv(env, "AUTO_MIGRATE", true),
		ReadOnly:     boolEnv(env, "READ_ONLY", false),
		WebUI:        boolEnv(env, "WEB", true),
		APIURL:       strings.TrimRight(env("CONSPECTUS_API_URL"), "/"),
		LogLevel:     get(env, "LOG_LEVEL", DefaultLogLevel),
		LogFormat:    get(env, "LOG_FORMAT", DefaultLogFormat),
		LogColours:   boolEnv(env, "LOG_COLOURS", true),
	}

	// Fall back to the repo's ./migrations only when the image default is
	// absent (bare binary run from a checkout) - same rule as the web dir.
	if c.MigrationDir == DefaultMigrationsDir && !dirExists(c.MigrationDir) {
		c.MigrationDir = MigrationsDirFallback
	}
	// Fall back to the repo's ./web only when the image default is absent
	// (bare binary run from a checkout); an explicit CONSPECTUS_WEB_DIR
	// always wins, missing directory or not (the render engine says so).
	if c.WebDir == DefaultWebDir && !dirExists(c.WebDir) {
		c.WebDir = WebDirFallback
	}

	loc, err := time.LoadLocation(c.TimezoneName)
	if err != nil {
		return c, fmt.Errorf("config: invalid TZ %q: %w", c.TimezoneName, err)
	}
	c.Timezone = loc

	switch c.LogFormat {
	case "json", "text":
	default:
		return c, fmt.Errorf("config: LOG_FORMAT must be json or text, got %q", c.LogFormat)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return c, fmt.Errorf("config: LOG_LEVEL must be debug|info|warn|error, got %q", c.LogLevel)
	}
	return c, nil
}

func get(env EnvFn, suffix, def string) string {
	if v := env("CONSPECTUS_" + suffix); v != "" {
		return v
	}
	return def
}

func boolEnv(env EnvFn, suffix string, def bool) bool {
	v := env("CONSPECTUS_" + suffix)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
