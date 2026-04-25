package config

import "os"

type Config struct {
	APIBaseURL   string
	DefaultLang  string
	Username     string
	Token        string
	ServiceToken string
}

func Load() Config {
	return Config{
		APIBaseURL:   getEnv("BOT_API_BASE_URL", "http://localhost:8080"),
		DefaultLang:  getEnv("BOT_DEFAULT_LANG", "ru"),
		Username:     getEnv("BOT_USERNAME", "your_bot_username"),
		Token:        os.Getenv("BOT_TOKEN"),
		ServiceToken: getEnv("BOT_SERVICE_TOKEN", "change_me_bot_service_token"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
