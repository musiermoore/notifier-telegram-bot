package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/alexandersustavov/notifier/notifier-telegram-bot/internal/config"
)

func main() {
	cfg := config.Load()
	if cfg.Token == "" {
		log.Printf("notifier-telegram-bot bootstrap api=%s lang=%s token_set=%t", cfg.APIBaseURL, cfg.DefaultLang, false)
		return
	}

	log.Printf("notifier-telegram-bot running api=%s lang=%s username=%s", cfg.APIBaseURL, cfg.DefaultLang, cfg.Username)

	client := &http.Client{Timeout: 30 * time.Second}
	ctx := context.Background()
	if err := deleteWebhook(ctx, client, cfg.Token); err != nil {
		log.Printf("delete webhook: %v", err)
	}
	var offset int64

	for {
		if err := deliverPendingReminders(ctx, client, cfg); err != nil {
			log.Printf("deliver reminders: %v", err)
		}

		updates, err := getUpdates(ctx, client, cfg.Token, offset)
		if err != nil {
			log.Printf("get updates: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.CallbackQuery != nil {
				if err := handleCallbackQuery(ctx, client, cfg, *update.CallbackQuery); err != nil {
					log.Printf("handle callback query: %v", err)
				}
				continue
			}

			if update.Message == nil {
				continue
			}

			if err := handleMessage(ctx, client, cfg, *update.Message); err != nil {
				log.Printf("handle message: %v", err)
			}
		}
	}
}
