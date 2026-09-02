package storage

import (
	"strings"
	"testing"

	"conspectus/internal/config"
	"conspectus/internal/domain"
)

func TestDriverSelfDescription(t *testing.T) {
	RegisterDriver(DriverInfo{
		Name: "testdrv",
		EnvVars: []EnvVar{
			{Key: "TEST_REQUIRED", Required: true, Purpose: "a required variable"},
			{Key: "TEST_OPTIONAL", Default: "x", Purpose: "an optional variable"},
		},
	}, func(cfg config.Config) (domain.Repos, error) { return nil, nil })

	vars := EnvVars("testdrv")
	if len(vars) != 2 || vars[0].Key != "TEST_REQUIRED" || !vars[0].Required {
		t.Fatalf("EnvVars = %+v", vars)
	}
	if EnvVars("never-registered") != nil {
		t.Fatal("unknown driver must report no vars")
	}

	if err := CheckEnvLookup("testdrv", func(k string) string {
		return map[string]string{"TEST_REQUIRED": "x"}[k]
	}); err != nil {
		t.Fatalf("satisfied env rejected: %v", err)
	}

	err := CheckEnvLookup("testdrv", func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "TEST_REQUIRED") {
		t.Fatalf("want missing TEST_REQUIRED error, got %v", err)
	}

	if err := CheckEnvLookup("never-registered", func(string) string { return "" }); err != nil {
		t.Fatalf("unknown driver must pass pre-flight, got %v", err)
	}
}
