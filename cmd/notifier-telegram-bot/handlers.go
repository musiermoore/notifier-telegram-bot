package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alexandersustavov/notifier/notifier-telegram-bot/internal/config"
)

var hexCodePattern = regexp.MustCompile(`^[A-Fa-f0-9]{8}$`)

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
		fmt.Sprintf("Подключенный аккаунт:\nID: %s\nИмя: %s\nEmail: %s%s", user.accountID(), user.Name, user.Email, username))
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

	return sendListPage(ctx, client, cfg, message.Chat.ID, 0, response.Items, false, 0)
}

func handleCallbackQuery(ctx context.Context, client *http.Client, cfg config.Config, query telegramCallbackQuery) error {
	defer func() {
		if err := answerCallbackQuery(ctx, client, cfg.Token, query.ID, ""); err != nil {
			log.Printf("answer callback query: %v", err)
		}
	}()

	if query.Message == nil {
		return nil
	}

	action, page, index, ok := parseListCallback(query.Data)
	if !ok {
		return nil
	}

	response, err := fetchTelegramItems(ctx, client, cfg, query.Message.Chat.ID)
	if err != nil {
		return editMessageText(ctx, client, cfg.Token, query.Message.Chat.ID, query.Message.MessageID,
			"Не удалось загрузить записи. Попробуй /list еще раз.", nil)
	}

	switch action {
	case "page":
		return sendListPage(ctx, client, cfg, query.Message.Chat.ID, page, response.Items, true, query.Message.MessageID)
	case "item":
		return sendListItemDetails(ctx, client, cfg, query.Message.Chat.ID, query.Message.MessageID, page, index, response.Items)
	default:
		return nil
	}
}

func sendListPage(ctx context.Context, client *http.Client, cfg config.Config, chatID int64, page int, items []telegramAPIItem, edit bool, messageID int64) error {
	page = clampListPage(page, len(items))
	text, markup := buildListPage(page, items)
	if edit {
		return editMessageText(ctx, client, cfg.Token, chatID, messageID, text, markup)
	}

	return sendMessageWithMarkup(ctx, client, cfg.Token, chatID, text, markup)
}

func sendListItemDetails(ctx context.Context, client *http.Client, cfg config.Config, chatID, messageID int64, page, index int, items []telegramAPIItem) error {
	if index < 0 || index >= len(items) {
		return sendListPage(ctx, client, cfg, chatID, page, items, true, messageID)
	}

	item := items[index]
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = fmt.Sprintf("Без названия %d", index+1)
	}

	lines := []string{fmt.Sprintf("<b>%s</b>", html.EscapeString(title))}
	if scheduledAt := formatScheduledAt(item.RemindAt); scheduledAt != "" {
		lines = append(lines, fmt.Sprintf("Запланировано на %s", scheduledAt))
	}
	if strings.TrimSpace(item.Body) != "" {
		lines = append(lines, "", html.EscapeString(strings.TrimSpace(item.Body)))
	}

	markup := &telegramInlineKeyboardMarkup{
		InlineKeyboard: [][]telegramInlineKeyboardButton{
			{
				{Text: "Назад", CallbackData: fmt.Sprintf("list:page:%d", page)},
			},
		},
	}

	return editMessageTextHTML(ctx, client, cfg.Token, chatID, messageID, strings.Join(lines, "\n"), markup)
}

func formatScheduledAt(remindAt *string) string {
	if remindAt == nil {
		return ""
	}

	value := strings.TrimSpace(*remindAt)
	if value == "" {
		return ""
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("02/01/2006 в 15:04")
		}
	}

	return value
}

func buildListPage(page int, items []telegramAPIItem) (string, *telegramInlineKeyboardMarkup) {
	page = clampListPage(page, len(items))
	start := page * listPageSize
	end := start + listPageSize
	if end > len(items) {
		end = len(items)
	}

	rows := make([][]telegramInlineKeyboardButton, 0, (end-start)+1)
	for index := start; index < end; index++ {
		title := strings.TrimSpace(items[index].Title)
		if title == "" {
			title = fmt.Sprintf("Без названия %d", index+1)
		}
		rows = append(rows, []telegramInlineKeyboardButton{
			{Text: title, CallbackData: fmt.Sprintf("list:item:%d:%d", page, index)},
		})
	}

	navRow := make([]telegramInlineKeyboardButton, 0, 2)
	if page > 0 {
		navRow = append(navRow, telegramInlineKeyboardButton{
			Text:         "Назад",
			CallbackData: fmt.Sprintf("list:page:%d", page-1),
		})
	}
	if end < len(items) {
		navRow = append(navRow, telegramInlineKeyboardButton{
			Text:         "Дальше",
			CallbackData: fmt.Sprintf("list:page:%d", page+1),
		})
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	totalPages := (len(items) + listPageSize - 1) / listPageSize
	if totalPages == 0 {
		totalPages = 1
	}

	return fmt.Sprintf("Последние записи:\nСтраница %d из %d", page+1, totalPages), &telegramInlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

func clampListPage(page, total int) int {
	if page < 0 {
		return 0
	}
	if total <= 0 {
		return 0
	}

	lastPage := (total - 1) / listPageSize
	if page > lastPage {
		return lastPage
	}

	return page
}

func parseListCallback(data string) (action string, page int, index int, ok bool) {
	var parsedPage int
	var parsedIndex int

	switch {
	case strings.HasPrefix(data, "list:page:"):
		if _, err := fmt.Sscanf(data, "list:page:%d", &parsedPage); err != nil {
			return "", 0, 0, false
		}
		return "page", parsedPage, 0, true
	case strings.HasPrefix(data, "list:item:"):
		if _, err := fmt.Sscanf(data, "list:item:%d:%d", &parsedPage, &parsedIndex); err != nil {
			return "", 0, 0, false
		}
		return "item", parsedPage, parsedIndex, true
	default:
		return "", 0, 0, false
	}
}
