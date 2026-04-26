package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexandersustavov/notifier/notifier-telegram-bot/internal/config"
)

func deliverPendingReminders(ctx context.Context, client *http.Client, cfg config.Config) error {
	deliveries, err := fetchPendingDeliveries(ctx, client, cfg)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		chatID := strings.TrimSpace(delivery.ChatID)
		if chatID == "" {
			continue
		}

		if err := sendMessageByChatID(ctx, client, cfg.Token, chatID, delivery.Message); err != nil {
			log.Printf("telegram delivery failed id=%s: %v", delivery.ID, err)
			_ = markDeliveryFailed(ctx, client, cfg, delivery.ID, err.Error())
			continue
		}

		if err := markDeliveryComplete(ctx, client, cfg, delivery.ID); err != nil {
			log.Printf("mark delivery complete failed id=%s: %v", delivery.ID, err)
		}
	}

	return nil
}

func fetchPendingDeliveries(ctx context.Context, client *http.Client, cfg config.Config) ([]telegramDelivery, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(cfg.APIBaseURL, "/")+"/v1/internal/telegram/deliveries/pending",
		nil,
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Service-Token", cfg.ServiceToken)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("pending deliveries status %d: %s", response.StatusCode, string(body))
	}

	var payload pendingDeliveriesResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return payload.Deliveries, nil
}

func fetchTelegramMe(ctx context.Context, client *http.Client, cfg config.Config, chatID int64) (apiUser, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(cfg.APIBaseURL, "/")+"/v1/internal/telegram/me?chat_id="+url.QueryEscape(fmt.Sprintf("%d", chatID)),
		nil,
	)
	if err != nil {
		return apiUser{}, err
	}
	request.Header.Set("X-Service-Token", cfg.ServiceToken)

	response, err := client.Do(request)
	if err != nil {
		return apiUser{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return apiUser{}, fmt.Errorf("telegram me status %d: %s", response.StatusCode, string(body))
	}

	var payload meResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return apiUser{}, err
	}

	return payload.User, nil
}

func fetchTelegramItems(ctx context.Context, client *http.Client, cfg config.Config, chatID int64) (listItemsResponse, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(cfg.APIBaseURL, "/")+"/v1/internal/telegram/items?chat_id="+url.QueryEscape(fmt.Sprintf("%d", chatID)),
		nil,
	)
	if err != nil {
		return listItemsResponse{}, err
	}
	request.Header.Set("X-Service-Token", cfg.ServiceToken)

	response, err := client.Do(request)
	if err != nil {
		return listItemsResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return listItemsResponse{}, fmt.Errorf("telegram items status %d: %s", response.StatusCode, string(body))
	}

	var payload listItemsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return listItemsResponse{}, err
	}

	return payload, nil
}

func markDeliveryComplete(ctx context.Context, client *http.Client, cfg config.Config, deliveryID string) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(cfg.APIBaseURL, "/")+"/v1/internal/telegram/deliveries/"+deliveryID+"/complete",
		bytes.NewReader([]byte(`{}`)),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Service-Token", cfg.ServiceToken)

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("mark complete status %d: %s", response.StatusCode, string(body))
	}

	return nil
}

func markDeliveryFailed(ctx context.Context, client *http.Client, cfg config.Config, deliveryID, reason string) error {
	body, err := json.Marshal(map[string]string{"error": reason})
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(cfg.APIBaseURL, "/")+"/v1/internal/telegram/deliveries/"+deliveryID+"/fail",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Service-Token", cfg.ServiceToken)

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return fmt.Errorf("mark failed status %d: %s", response.StatusCode, string(payload))
	}

	return nil
}

func consumeLinkCode(ctx context.Context, client *http.Client, apiBaseURL, code string, chatID int64, user *telegramUser) error {
	var username *string
	if user != nil {
		username = user.Username
	}

	body, err := json.Marshal(map[string]any{
		"code":     code,
		"chat_id":  fmt.Sprintf("%d", chatID),
		"username": username,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(apiBaseURL, "/")+"/v1/telegram/consume-link", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return fmt.Errorf("consume link status %d: %s", response.StatusCode, string(payload))
	}

	return nil
}
