package storage

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"conspectus/internal/config"
	"conspectus/internal/domain"
)

type Factory func(cfg config.Config) (domain.Repos, error)

type EnvVar struct {
	Key      string
	Required bool
	Default  string
	Purpose  string
}

type DriverInfo struct {
	Name    string
	EnvVars []EnvVar
}

type entry struct {
	open Factory
	info DriverInfo
}

var (
	mu      sync.RWMutex
	drivers = map[string]entry{}
)

func RegisterDriver(info DriverInfo, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	drivers[info.Name] = entry{open: f, info: info}
}

func Drivers() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(drivers))
	for n := range drivers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func EnvVars(driver string) []EnvVar {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := drivers[driver]
	if !ok {
		return nil
	}
	return e.info.EnvVars
}

func CheckEnv(driver string) error { return CheckEnvLookup(driver, os.Getenv) }

func CheckEnvLookup(driver string, get func(string) string) error {
	mu.RLock()
	e, ok := drivers[driver]
	mu.RUnlock()
	if !ok {
		return nil
	}
	for _, v := range e.info.EnvVars {
		if v.Required && get(v.Key) == "" {
			return fmt.Errorf("storage: driver %q needs %s (%s)", driver, v.Key, v.Purpose)
		}
	}
	return nil
}

func Open(cfg config.Config) (domain.Repos, error) {
	mu.RLock()
	e, ok := drivers[cfg.Driver]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("storage: unknown driver %q (registered: %v)", cfg.Driver, Drivers())
	}
	return e.open(cfg)
}
