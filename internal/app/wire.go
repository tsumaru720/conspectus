package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"conspectus/internal/apiclient"
	"conspectus/internal/config"
	"conspectus/internal/domain"
	"conspectus/internal/httpapi"
	"conspectus/internal/importer"
	"conspectus/internal/migrations"
	"conspectus/internal/render"
	"conspectus/internal/settings"
	"conspectus/internal/stats"
	"conspectus/internal/views"
	"conspectus/internal/web"
)

type App struct {
	Cfg      config.Config
	Log      *slog.Logger
	Info     BuildInfo
	DB       *sql.DB
	Repos    domain.Repos
	Settings *settings.Service
	Router   *httpapi.Router
	API      *httpapi.API
	Render   *render.Engine
	Stats    *stats.Service
	Importer *importer.Importer
	Pages    *web.Pages

	srv *http.Server
}

type reposHolder struct {
	mu    sync.RWMutex
	repos domain.Repos
}

func (h *reposHolder) get() (domain.Repos, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.repos == nil {
		return nil, fmt.Errorf("storage not initialised yet")
	}
	return h.repos, nil
}

func (h *reposHolder) set(repos domain.Repos) {
	h.mu.Lock()
	h.repos = repos
	h.mu.Unlock()
}

func New(cfg config.Config, log *slog.Logger, info BuildInfo) (*App, error) {
	holder := &reposHolder{}
	a := &App{
		Cfg:  cfg,
		Log:  log,
		Info: info,
	}

	a.Settings = settings.New(log, func() (domain.SettingsRepo, error) {
		repos, err := holder.get()
		if err != nil {
			return nil, err
		}
		return repos.Settings(), nil
	})
	a.Router = httpapi.NewRouter()
	httpapi.InstallCoreMiddleware(a.Router, httpapi.MiddlewareDeps{
		Log:      log,
		Settings: a.Settings,
		ReadOnly: cfg.ReadOnly,
	})
	// One page render composes several API fetches; the request cache in the
	// context lets repeated GETs of the same URL within a single render share
	// one round-trip. API paths skip it - loopback calls already inherit the
	// page's cache through its context.
	if err := a.Router.AddMiddleware("core:reqcache", -650, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				r = r.WithContext(apiclient.WithRequestCache(r.Context()))
			}
			next.ServeHTTP(w, r)
		})
	}); err != nil {
		return nil, fmt.Errorf("request cache middleware: %w", err)
	}
	if !cfg.WebUI {
		err := a.Router.AddMiddleware("core:weboff", -600, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := r.URL.Path
				if strings.HasPrefix(p, "/api/") || p == "/healthz" || p == "/readyz" {
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintln(w, `{"error":{"code":"web_disabled","message":"web frontend disabled (CONSPECTUS_WEB=false)"}}`)
			})
		})
		if err != nil {
			return nil, err
		}
	}

	a.Stats = &stats.Service{
		Repos:    holder.get,
		Settings: a.Settings,
		Log:      log,
		Now:      time.Now,
	}

	var apiC *apiclient.Client
	frontendOnly := cfg.APIURL != ""
	if frontendOnly {
		apiC = apiclient.NewRemote(cfg.APIURL)
		log.Info("frontend targets remote API", "url", cfg.APIURL)
	} else {
		apiC = apiclient.NewLoopback(a.Router)
	}
	frontSettings := web.NewSettingsViaAPI(apiC)

	a.Render = render.New(cfg.WebDir, log, frontSettings)
	a.Render.Version = info.Version
	a.Render.ReadOnly = cfg.ReadOnly

	a.Importer = importer.New(apiC, cfg.Timezone)

	schemaVer := migrations.ExpectedVersion(migrationSet(&cfg))
	a.API = &httpapi.API{
		Router:   a.Router,
		Settings: a.Settings,
		Stats:    statsOrNil(a.Stats, frontendOnly),
		Log:      log,
		Loc:      cfg.Timezone,
		ReadOnly: cfg.ReadOnly,
		Info: httpapi.SystemInfo{
			Version:               info.Version,
			Commit:                info.Commit,
			ExpectedSchemaVersion: schemaVer,
		},
		Repos: holder.get,
	}
	if frontendOnly {
		if err := a.Router.Register("core:healthz", "GET", "/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("ok (frontend)"))
		})); err != nil {
			return nil, fmt.Errorf("core routes: %w", err)
		}
	} else if err := a.API.RegisterCoreRoutes(); err != nil {
		return nil, fmt.Errorf("core routes: %w", err)
	}
	a.Pages = &web.Pages{
		Router:   a.Router,
		Render:   a.Render,
		Settings: frontSettings,
		Log:      log,
		Importer: a.Importer,
		API:      apiC,
		Loc:      cfg.Timezone,
		Now:      time.Now,
		Version:  info.Version,
	}
	if !cfg.WebUI {
		log.Info("web frontend disabled (CONSPECTUS_WEB=false) - serving API only")
	} else if err := a.Pages.Register(); err != nil {
		return nil, fmt.Errorf("core pages: %w", err)
	}
	if cfg.WebUI {
		if err := views.Register(views.Deps{
			Router:   a.Router,
			Render:   a.Render,
			API:      apiC,
			Settings: a.Settings,
			Log:      log,
		}); err != nil {
			return nil, fmt.Errorf("views: %w", err)
		}
	}
	if !frontendOnly {
		a.API.SetErrorLogger()
	}
	if frontendOnly {
		err := a.Router.AddMiddleware("core:apifront", 0, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := r.URL.Path
				local := strings.HasPrefix(p, "/api/ui/v1/")
				if strings.HasPrefix(p, "/api/") && !local {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintln(w, `{"error":{"code":"not_found","message":"no such local endpoint; the data API runs on the API server"}}`)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		if err != nil {
			return nil, fmt.Errorf("api frontend gate: %w", err)
		}
	}

	if !frontendOnly {
		db, repos, err := openRepos(cfg, log)
		if err != nil {
			return nil, err
		}
		a.DB = db
		a.Repos = repos
		holder.set(repos)
	}

	log.Info("composition complete",
		"routes", len(a.Router.Routes()),
		"webdir", a.Render.Dir,
		"mode", map[bool]string{true: "frontend (remote API)", false: "bundled"}[frontendOnly],
	)
	return a, nil
}

func statsOrNil(s *stats.Service, frontendOnly bool) *stats.Service {
	if frontendOnly {
		return nil
	}
	return s
}

func (a *App) AutoMigrate() error {
	set := migrationSet(&a.Cfg)
	runner := &migrations.Runner{DB: a.DB, Logger: a.Log, Set: set}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := runner.Run(ctx)
	if err != nil {
		return err
	}
	a.Log.Info("auto-migrate complete", "from", res.From, "to", res.To)
	return nil
}

func (a *App) CheckMigrationsCurrent() error {
	set := migrationSet(&a.Cfg)
	expected := migrations.ExpectedVersion(set)
	runner := &migrations.Runner{DB: a.DB, Logger: a.Log, Set: set}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	current, err := runner.CurrentVersion(ctx)
	if err != nil {
		return err
	}
	if current < expected {
		return fmt.Errorf("schema version %d is stale (expected %d) - run `conspectus migrate` (or clear CONSPECTUS_AUTO_MIGRATE=false to migrate on startup); refusing to serve", current, expected)
	}
	return nil
}

func (a *App) Close() {
	if a.Repos != nil {
		a.Repos.Close()
	}
}

func (a *App) Serve() error {
	a.srv = &http.Server{
		Addr:              config.DefaultListen,
		Handler:           a.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() {
		a.Log.Info("listening", "addr", config.DefaultListen)
		if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-stop:
		a.Log.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a.srv.Shutdown(ctx)
	}
}
