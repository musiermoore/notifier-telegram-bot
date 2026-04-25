package config

import "os"

type Config struct {
	APIBaseURL  string
	DefaultLang string
	Token       string
}

func Load() Config {
	return Config{
		APIBaseURL:  getEnv("BOT_API_BASE_URL", "http://localhost:8080"),
		DefaultLang: getEnv("BOT_DEFAULT_LANG", "ru"),
		Token:       os.Getenv("BOT_TOKEN"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
