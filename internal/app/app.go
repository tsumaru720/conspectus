package app

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"

	rootembed "conspectus"
	"conspectus/internal/config"
	"conspectus/internal/domain"
	"conspectus/internal/migrations"
	"conspectus/internal/storage"
	_ "conspectus/internal/storage/mysql"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type RawDBExposer interface {
	RawDB() *sql.DB
}

func Run(args []string, info BuildInfo) error {
	// No arguments means the server: `conspectus` alone is the common case
	// (and the container entrypoint), explicit subcommands opt into the rest.
	if len(args) == 0 {
		return runServe(nil, info)
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:], info)
	case "migrate":
		return runMigrate(args[1:], info)
	case "version", "--version", "-v":
		printVersion(info)
		return nil
	case "help", "--help", "-h":
		usage(info)
		return nil
	default:
		return fmt.Errorf("unknown command %q (try: migrate | version)", args[0])
	}
}

func usage(info BuildInfo) {
	fmt.Fprintf(os.Stderr, `conspectus %s - personal wealth tracker

Usage:
  conspectus                        start the HTTP server (default)
  conspectus migrate                apply pending migrations
  conspectus migrate --plan         dry-run: list pending migrations
  conspectus migrate --repair       resume a failed migration from its recorded offset
  conspectus version                build info

Existing Conspectus v1 databases upgrade in place: `+"`conspectus migrate`"+` resumes
the migration chain from the database's stored db_version (serving applies
pending migrations automatically; set CONSPECTUS_AUTO_MIGRATE=false to
require the explicit command). Environment: CONSPECTUS_* variables
(see README.md). Storage drivers declare the variables they consume:
`, info.Version)
	for _, name := range storage.Drivers() {
		fmt.Fprintf(os.Stderr, "  %s\n", name)
		for _, v := range storage.EnvVars(name) {
			def := ""
			if v.Default != "" {
				def = " [default: " + v.Default + "]"
			}
			req := ""
			if v.Required {
				req = " (required)"
			}
			fmt.Fprintf(os.Stderr, "    %s%s%s - %s\n", v.Key, req, def, v.Purpose)
		}
	}
}

func printVersion(info BuildInfo) {
	set := migrationSet(nil)
	fmt.Printf("conspectus %s (commit %s, built %s)\n", info.Version, info.Commit, info.Date)
	fmt.Printf("expected schema version: %d\n", migrations.ExpectedVersion(set))
}

func migrationSet(cfg *config.Config) []migrations.Migration {
	var sources []fs.FS
	var names []string
	emb, err := rootembed.EmbeddedMigrations()
	if err == nil {
		sources = append(sources, emb)
		names = append(names, "embedded")
	}
	if cfg != nil {
		if st, err := os.Stat(cfg.MigrationDir); err == nil && st.IsDir() {
			sources = append(sources, os.DirFS(cfg.MigrationDir))
			names = append(names, cfg.MigrationDir)
		}
	}
	set, err := migrations.Load(sources, names)
	if err != nil {
		return nil
	}
	return set
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.LogFormat == "text" {
		var w io.Writer = os.Stderr
		if cfg.LogColours {
			w = &colourWriter{w: os.Stderr}
		}
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

func loadConfigForDB() (config.Config, *slog.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, nil, err
	}
	return cfg, newLogger(cfg), nil
}

func openRepos(cfg config.Config, log *slog.Logger) (*sql.DB, domain.Repos, error) {
	repos, err := storage.Open(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("storage: %w", err)
	}
	db := rawDB(repos)
	if db == nil {
		repos.Close()
		return nil, nil, fmt.Errorf("storage driver %q does not expose a raw DB handle; migrations unsupported", cfg.Driver)
	}
	log.Info("storage open", "driver", cfg.Driver)
	return db, repos, nil
}

func rawDB(repos domain.Repos) *sql.DB {
	if ex, ok := repos.(RawDBExposer); ok {
		return ex.RawDB()
	}
	return nil
}

func runMigrate(args []string, info BuildInfo) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	plan := fs.Bool("plan", false, "dry-run: list pending migrations and exit")
	repair := fs.Bool("repair", false, "resume the recorded failed migration from its statement offset")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, log, err := loadConfigForDB()
	if err != nil {
		return err
	}
	db, repos, err := openRepos(cfg, log)
	if err != nil {
		return err
	}
	defer repos.Close()

	set := migrationSet(&cfg)
	if len(set) == 0 {
		return fmt.Errorf("no migrations found (embedded missing?)")
	}
	runner := &migrations.Runner{DB: db, Logger: log, Set: set}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	switch {
	case *plan:
		pending, current, err := runner.Plan(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("current schema version: %d\n", current)
		fmt.Printf("expected schema version: %d\n", migrations.ExpectedVersion(set))
		if len(pending) == 0 {
			fmt.Println("nothing to apply - up to date")
			return nil
		}
		for _, m := range pending {
			stmts := 1
			if !m.Raw {
				stmts = len(migrations.Statements(m.Contents))
			}
			fmt.Printf("  %04d  %-40s (%d statements, %s)\n", m.Version, m.Name, stmts, m.Source)
		}
		return nil

	case *repair:
		res, err := runner.Repair(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("repaired: schema version %d → %d (applied: %s)\n", res.From, res.To, versionList(res.Applied))
		return nil

	default:
		res, err := runner.Run(ctx)
		if err != nil {
			return err
		}
		if len(res.Applied) == 0 {
			fmt.Printf("nothing to apply - schema version %d, up to date\n", res.To)
			return nil
		}
		fmt.Printf("applied: %s (schema version %d → %d)\n", versionList(res.Applied), res.From, res.To)
		return nil
	}
}

func versionList(vs []int) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = fmt.Sprintf("%04d", v)
	}
	return strings.Join(parts, ", ")
}

func runServe(args []string, info BuildInfo) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.APIURL == "" {
		if err := storage.CheckEnv(cfg.Driver); err != nil {
			return err
		}
	}
	log := newLogger(cfg)

	a, err := New(cfg, log, info)
	if err != nil {
		return err
	}
	defer a.Close()

	if cfg.APIURL != "" {
		log.Info("frontend-only mode: skipping local storage and migration gate", "api", cfg.APIURL)
	} else if cfg.AutoMigrate {
		if err := a.AutoMigrate(); err != nil {
			return fmt.Errorf("auto-migrate: %w", err)
		}
	} else if err := a.CheckMigrationsCurrent(); err != nil {
		return err
	}

	return a.Serve()
}
