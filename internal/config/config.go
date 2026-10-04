package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIKey         string
	BaseURL        string
	Model          string
	RequestTimeout time.Duration
	MaxAgentSteps  int
}

func Load() (Config, error) {
	_ = loadDotEnv(".env")

	cfg := Config{
		APIKey:         os.Getenv("DEEPSEEK_API_KEY"),
		BaseURL:        envOrDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		Model:          envOrDefault("DEEPSEEK_MODEL", "deepseek-flash"),
		RequestTimeout: 2 * time.Minute,
		MaxAgentSteps:  5,
	}
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("DEEPSEEK_API_KEY is required")
	}

	if raw := os.Getenv("REQUEST_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
		}
		cfg.RequestTimeout = d
	}
	if raw := os.Getenv("MAX_AGENT_STEPS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("MAX_AGENT_STEPS must be a positive integer")
		}
		cfg.MaxAgentSteps = n
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
