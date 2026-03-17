package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"as/kernel/internal/realizations"
)

type RuntimeConfig struct {
	DatabaseURL       string
	AutoMigrate       bool
	RateLimitsEnabled bool
	RateLimitWindow   int
	RateLimitBlock    int
	AnonymousRead     int
	AnonymousWrite    int
	SessionRead       int
	SessionWrite      int
	Internal          int
	Worker            int
	Feedback          int
}

func LoadRuntimeConfigFromEnv() RuntimeConfig {
	return RuntimeConfig{
		DatabaseURL:       strings.TrimSpace(os.Getenv("AS_RUNTIME_DATABASE_URL")),
		AutoMigrate:       envBool("AS_RUNTIME_AUTO_MIGRATE", false),
		RateLimitsEnabled: envBool("AS_RATE_LIMIT_ENABLED", true),
		RateLimitWindow:   envInt("AS_RATE_LIMIT_WINDOW_SECONDS", 60),
		RateLimitBlock:    envInt("AS_RATE_LIMIT_BLOCK_SECONDS", 60),
		AnonymousRead:     envInt("AS_RATE_LIMIT_ANONYMOUS_READ_LIMIT", 120),
		AnonymousWrite:    envInt("AS_RATE_LIMIT_ANONYMOUS_WRITE_LIMIT", 20),
		SessionRead:       envInt("AS_RATE_LIMIT_SESSION_READ_LIMIT", 240),
		SessionWrite:      envInt("AS_RATE_LIMIT_SESSION_WRITE_LIMIT", 60),
		Internal:          envInt("AS_RATE_LIMIT_INTERNAL_LIMIT", 180),
		Worker:            envInt("AS_RATE_LIMIT_WORKER_LIMIT", 1200),
		Feedback:          envInt("AS_RATE_LIMIT_FEEDBACK_LIMIT", 30),
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

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
