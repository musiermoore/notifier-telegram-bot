package main

import (
	"log"

	"github.com/alexandersustavov/notifier/notifier-telegram-bot/internal/config"
)

func main() {
	cfg := config.Load()
	log.Printf("notifier-telegram-bot bootstrap api=%s lang=%s token_set=%t",
		cfg.APIBaseURL,
		cfg.DefaultLang,
		cfg.Token != "",
	)
}
