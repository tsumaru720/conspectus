package settings

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"conspectus/internal/domain"
)

const (
	KeyProjectionTgt   = "projection_targets"
	KeyProjectionCurve = "projection_target_curve"
)

const DefaultProjectionCurve = `{"value":1000000,"year":2048}`

type Service struct {
	provider  func() (domain.SettingsRepo, error)
	log       *slog.Logger
	mu        sync.RWMutex
	cache     map[string]string
	loaded    bool
	observers []func(key, value string)
}

func New(log *slog.Logger, provider func() (domain.SettingsRepo, error)) *Service {
	return &Service{provider: provider, log: log, cache: map[string]string{}}
}

func (s *Service) OnChange(fn func(key, value string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, fn)
}

func (s *Service) Get(key string) string {
	if v, ok := s.peek(key); ok {
		return v
	}
	return Default(key)
}

func (s *Service) peek(key string) (string, bool) {
	if err := s.ensureLoaded(); err != nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.cache[key]
	return v, ok
}

func (s *Service) Set(ctx context.Context, key, value string) error {
	if err := Validate(key, value); err != nil {
		return err
	}
	repo, err := s.provider()
	if err != nil {
		return err
	}
	if err := repo.Set(ctx, key, value); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[key] = value
	observers := append([]func(string, string){}, s.observers...)
	s.mu.Unlock()
	for _, fn := range observers {
		fn(key, value)
	}
	return nil
}

// Create inserts a new setting row with its value and display metadata;
// the key must not exist yet (the repo reports duplicates as ErrConflict).
// On success the value joins the cache and observers fire, like Set.
func (s *Service) Create(ctx context.Context, key, value, description string, display bool) error {
	if err := Validate(key, value); err != nil {
		return err
	}
	repo, err := s.provider()
	if err != nil {
		return err
	}
	if err := repo.Create(ctx, key, value, description, display); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[key] = value
	observers := append([]func(string, string){}, s.observers...)
	s.mu.Unlock()
	for _, fn := range observers {
		fn(key, value)
	}
	return nil
}

// SetDescription updates a setting's description without touching its
// value. Descriptions are not cached, so this goes straight to the repo.
func (s *Service) SetDescription(ctx context.Context, key, description string) error {
	repo, err := s.provider()
	if err != nil {
		return err
	}
	return repo.SetDescription(ctx, key, description)
}

// SetDisplay opts a setting in or out of the manage page. The display flag
// is not cached, so this goes straight to the repo.
func (s *Service) SetDisplay(ctx context.Context, key string, display bool) error {
	repo, err := s.provider()
	if err != nil {
		return err
	}
	return repo.SetDisplay(ctx, key, display)
}

// Delete removes a setting row and drops it from the cache; observers are
// notified with an empty value (reads fall back to the key's default).
func (s *Service) Delete(ctx context.Context, key string) error {
	repo, err := s.provider()
	if err != nil {
		return err
	}
	if err := repo.Delete(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, key)
	observers := append([]func(string, string){}, s.observers...)
	s.mu.Unlock()
	for _, fn := range observers {
		fn(key, "")
	}
	return nil
}

func (s *Service) ensureLoaded() error {
	s.mu.RLock()
	loaded := s.loaded
	s.mu.RUnlock()
	if loaded {
		return nil
	}
	repo, err := s.provider()
	if err != nil {
		return err
	}
	all, err := repo.All(context.Background())
	if err != nil {
		return fmt.Errorf("settings: load: %w", err)
	}
	s.mu.Lock()
	s.cache = all
	s.loaded = true
	s.mu.Unlock()
	return nil
}

func Default(key string) string {
	switch key {
	case KeyProjectionCurve:
		return DefaultProjectionCurve
	}
	return ""
}

func Validate(key, value string) error {
	if key == "" || len(key) > 20 {
		return fmt.Errorf("%w: settings key must be 1-20 characters", domain.ErrValidation)
	}
	if len(value) > 2048 {
		return fmt.Errorf("%w: settings value exceeds 2048 characters", domain.ErrValidation)
	}
	return nil
}

func IsSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, frag := range []string{"password", "secret", "token", "basicauth"} {
		if strings.Contains(k, frag) {
			return true
		}
	}
	return false
}
