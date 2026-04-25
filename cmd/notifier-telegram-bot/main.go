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
	"regexp"
	"strings"
	"time"

	"github.com/alexandersustavov/notifier/notifier-telegram-bot/internal/config"
)

type telegramUpdateResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text string        `json:"text"`
	Chat telegramChat  `json:"chat"`
	From *telegramUser `json:"from"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	Username *string `json:"username"`
}

type apiUser struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Name             string  `json:"name"`
	Lang             string  `json:"lang"`
	TelegramChat     *string `json:"telegram_chat,omitempty"`
	TelegramUsername *string `json:"telegram_username,omitempty"`
}

type meResponse struct {
	User apiUser `json:"user"`
}

type listItemsResponse struct {
	User  apiUser           `json:"user"`
	Items []telegramAPIItem `json:"items"`
}

type telegramAPIItem struct {
	Title             string  `json:"title"`
	Body              string  `json:"body"`
	RemindAt          *string `json:"remind_at"`
	DeliverToTelegram bool    `json:"deliver_to_telegram"`
}

type pendingDeliveriesResponse struct {
	Deliveries []telegramDelivery `json:"deliveries"`
}

type telegramDelivery struct {
	ID      string `json:"id"`
	ChatID  string `json:"chat_id"`
	Message string `json:"message"`
}

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
			if update.Message == nil {
				continue
			}

			if err := handleMessage(ctx, client, cfg, *update.Message); err != nil {
				log.Printf("handle message: %v", err)
			}
		}
	}
}

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

func deleteWebhook(ctx context.Context, client *http.Client, token string) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook?drop_pending_updates=false", token),
		nil,
	)
	if err != nil {
		return err
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("telegram deleteWebhook status %d: %s", response.StatusCode, string(body))
	}

	return nil
}

func handleMessage(ctx context.Context, client *http.Client, cfg config.Config, message telegramMessage) error {
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return nil
	}

	log.Printf("telegram message chat_id=%d text=%q", message.Chat.ID, text)

	code, isStartCommand := extractLinkCode(text)
	if isStartCommand || code != "" {
		if code == "" {
			return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
				"Открой Notifier в веб-интерфейсе, создай код привязки Telegram и отправь сюда /start ВАШ_КОД.")
		}

		if err := consumeLinkCode(ctx, client, cfg.APIBaseURL, code, message.Chat.ID, message.From); err != nil {
			return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
				"Не удалось привязать аккаунт. Проверь, что код еще действует, и при необходимости создай новый в веб-интерфейсе Notifier.")
		}

		return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
			"Telegram теперь подключен к твоему аккаунту Notifier. Вернись в веб-интерфейс и обнови статус.")
	}

	switch strings.ToLower(strings.Fields(text)[0]) {
	case "/me", "/me@oksananapominalabot":
		return handleMeCommand(ctx, client, cfg, message)
	case "/list", "/list@oksananapominalabot":
		return handleListCommand(ctx, client, cfg, message)
	}

	return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
		"Чтобы подключить этот Telegram-чат к Notifier, используй /start ВАШ_КОД или просто отправь сам код. Доступные команды: /me, /list.")
}

var hexCodePattern = regexp.MustCompile(`^[A-Fa-f0-9]{8}$`)

func extractLinkCode(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", false
	}

	first := strings.ToLower(fields[0])
	if strings.HasPrefix(first, "/start") {
		if len(fields) >= 2 {
			return strings.ToUpper(strings.TrimSpace(fields[1])), true
		}

		startCommand := strings.TrimSpace(trimmed[len(fields[0]):])
		if startCommand != "" {
			return strings.ToUpper(strings.TrimSpace(startCommand)), true
		}

		return "", true
	}

	if hexCodePattern.MatchString(trimmed) {
		return strings.ToUpper(trimmed), false
	}

	return "", false
}

func getUpdates(ctx context.Context, client *http.Client, token string, offset int64) ([]telegramUpdate, error) {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=25&offset=%d", token, offset)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram getUpdates status %d: %s", response.StatusCode, string(body))
	}

	var payload telegramUpdateResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	return payload.Result, nil
}

func sendMessage(ctx context.Context, client *http.Client, token string, chatID int64, text string) error {
	return sendMessageByChatID(ctx, client, token, fmt.Sprintf("%d", chatID), text)
}

func sendMessageByChatID(ctx context.Context, client *http.Client, token, chatID, text string) error {
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("telegram sendMessage status %d: %s", response.StatusCode, string(body))
	}

	return nil
}

func handleMeCommand(ctx context.Context, client *http.Client, cfg config.Config, message telegramMessage) error {
	user, err := fetchTelegramMe(ctx, client, cfg, message.Chat.ID)
	if err != nil {
		return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
			"Этот Telegram пока не привязан к аккаунту Notifier. Создай код в веб-интерфейсе и отправь /start ВАШ_КОД.")
	}

	username := ""
	if user.TelegramUsername != nil && strings.TrimSpace(*user.TelegramUsername) != "" {
		username = "\nTelegram: @" + strings.TrimSpace(*user.TelegramUsername)
	}

	return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
		fmt.Sprintf("Подключенный аккаунт:\nИмя: %s\nEmail: %s%s", user.Name, user.Email, username))
}

func handleListCommand(ctx context.Context, client *http.Client, cfg config.Config, message telegramMessage) error {
	response, err := fetchTelegramItems(ctx, client, cfg, message.Chat.ID)
	if err != nil {
		return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
			"Этот Telegram пока не привязан к аккаунту Notifier. Создай код в веб-интерфейсе и отправь /start ВАШ_КОД.")
	}

	if len(response.Items) == 0 {
		return sendMessage(ctx, client, cfg.Token, message.Chat.ID,
			"У тебя пока нет записей.")
	}

	lines := []string{"Последние записи:"}
	for index, item := range response.Items {
		line := fmt.Sprintf("%d. %s", index+1, item.Title)
		if item.RemindAt != nil && strings.TrimSpace(*item.RemindAt) != "" {
			line += fmt.Sprintf(" (напомнить в %s)", *item.RemindAt)
		}
		if item.DeliverToTelegram {
			line += " [TG]"
		}
		lines = append(lines, line)
		if strings.TrimSpace(item.Body) != "" {
			lines = append(lines, "   "+item.Body)
		}
	}

	return sendMessage(ctx, client, cfg.Token, message.Chat.ID, strings.Join(lines, "\n"))
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
