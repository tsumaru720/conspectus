package web

import (
	"context"
	"sync"
	"time"

	"conspectus/internal/apiclient"
	"conspectus/internal/domain"
)

// SettingsViaAPI is the frontend's short-TTL cache of the settings rows,
// fetched over the API (loopback or remote) so the web layer never touches
// storage directly.
type SettingsViaAPI struct {
	API *apiclient.Client
	TTL time.Duration

	mu      sync.Mutex
	rows    []domain.Setting
	fetched time.Time
}

func NewSettingsViaAPI(api *apiclient.Client) *SettingsViaAPI {
	return &SettingsViaAPI{API: api, TTL: 2 * time.Second}
}

// Rows returns the full settings list: key, value, description and display
// flag.
func (s *SettingsViaAPI) Rows() []domain.Setting {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresh()
	return s.rows
}

func (s *SettingsViaAPI) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresh()
	for _, row := range s.rows {
		if row.Key == key {
			return row.Value
		}
	}
	return ""
}

func (s *SettingsViaAPI) All() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresh()
	out := make(map[string]string, len(s.rows))
	for _, row := range s.rows {
		out[row.Key] = row.Value
	}
	return out
}

func (s *SettingsViaAPI) refresh() {
	if s.rows != nil && time.Since(s.fetched) <= s.TTL {
		return
	}
	var rows []domain.Setting
	if err := s.API.Get(context.Background(), "/api/v1/settings", &rows); err == nil {
		s.rows = rows
		s.fetched = time.Now()
	}
}
