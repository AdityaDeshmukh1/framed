package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all environment-derived configuration for the API service.
// Every field is populated at startup. If a required field is missing,
// the application exits — fail fast, not at runtime.
type Config struct {
	// App
	Env     string
	APIPort string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// External APIs
	TMDBApiKey      string
	AnthropicApiKey string
	OpenAIApiKey    string
	MLServiceURL    string

	// Auth
	JWTSecret string

	// Scraper behaviour
	ScraperRequestDelayMS int
	ScraperMaxConcurrent  int
}

// Load reads from .env (if present) then environment variables.
// Environment variables always win over .env — this means production
// can inject secrets via the runtime environment without a file.
func Load() (*Config, error) {
	// best-effort .env load — no error if file doesn't exist
	// in production there is no .env file, secrets come from the platform
	_ = godotenv.Load()

	cfg := &Config{
		Env:             getEnv("APP_ENV", "development"),
		APIPort:         getEnv("API_PORT", "8080"),
		DatabaseURL:     mustGetEnv("DATABASE_URL"),
		RedisURL:        mustGetEnv("REDIS_URL"),
		TMDBApiKey:      mustGetEnv("TMDB_API_KEY"),
		AnthropicApiKey: mustGetEnv("ANTHROPIC_API_KEY"),
		OpenAIApiKey:    mustGetEnv("OPENAI_API_KEY"),
		MLServiceURL:    getEnv("ML_SERVICE_URL", "http://localhost:8001"),
		JWTSecret:       mustGetEnv("JWT_SECRET"),
	}

	// parse int fields with defaults
	var err error
	cfg.ScraperRequestDelayMS, err = strconv.Atoi(getEnv("SCRAPER_REQUEST_DELAY_MS", "1200"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCRAPER_REQUEST_DELAY_MS: %w", err)
	}

	cfg.ScraperMaxConcurrent, err = strconv.Atoi(getEnv("SCRAPER_MAX_CONCURRENT", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCRAPER_MAX_CONCURRENT: %w", err)
	}

	return cfg, nil
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

// getEnv returns the env var value or a fallback default.
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// mustGetEnv returns the env var value or panics.
// Used for required config — we want a loud failure at startup,
// not a silent failure at the point of use.
func mustGetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return val
}
