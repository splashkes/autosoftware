package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"as/kernel/internal/realizations"
)

type RuntimeConfig struct {
	DatabaseURL string
	AutoMigrate bool
}

func LoadRuntimeConfigFromEnv() RuntimeConfig {
	return RuntimeConfig{
		DatabaseURL: strings.TrimSpace(os.Getenv("AS_RUNTIME_DATABASE_URL")),
		AutoMigrate: envBool("AS_RUNTIME_AUTO_MIGRATE", false),
	}
}

func (c RuntimeConfig) Enabled() bool {
	return c.DatabaseURL != ""
}

func RepoRootFromEnvOrWD() (string, error) {
	if root := strings.TrimSpace(os.Getenv("AS_REPO_ROOT")); root != "" {
		return root, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	root, err := realizations.FindRepoRoot(wd)
	if err != nil {
		return "", errors.New("resolve repo root: " + err.Error())
	}

	return root, nil
}

func EnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}

	return value
}
