package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

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
	return sendMessageWithMarkup(ctx, client, token, chatID, text, nil)
}

func sendMessageWithMarkup(ctx context.Context, client *http.Client, token string, chatID int64, text string, replyMarkup *telegramInlineKeyboardMarkup) error {
	return sendMessageByChatIDWithMarkup(ctx, client, token, fmt.Sprintf("%d", chatID), text, replyMarkup)
}

func sendMessageByChatID(ctx context.Context, client *http.Client, token, chatID, text string) error {
	return sendMessageByChatIDWithMarkup(ctx, client, token, chatID, text, nil)
}

func sendMessageByChatIDWithMarkup(ctx context.Context, client *http.Client, token, chatID, text string, replyMarkup *telegramInlineKeyboardMarkup) error {
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)
	if replyMarkup != nil {
		payload, err := json.Marshal(replyMarkup)
		if err != nil {
			return err
		}
		form.Set("reply_markup", string(payload))
	}

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

func editMessageText(ctx context.Context, client *http.Client, token string, chatID, messageID int64, text string, replyMarkup *telegramInlineKeyboardMarkup) error {
	return editMessageTextWithParseMode(ctx, client, token, chatID, messageID, text, replyMarkup, "")
}

func editMessageTextHTML(ctx context.Context, client *http.Client, token string, chatID, messageID int64, text string, replyMarkup *telegramInlineKeyboardMarkup) error {
	return editMessageTextWithParseMode(ctx, client, token, chatID, messageID, text, replyMarkup, "HTML")
}

func editMessageTextWithParseMode(ctx context.Context, client *http.Client, token string, chatID, messageID int64, text string, replyMarkup *telegramInlineKeyboardMarkup, parseMode string) error {
	form := url.Values{}
	form.Set("chat_id", fmt.Sprintf("%d", chatID))
	form.Set("message_id", fmt.Sprintf("%d", messageID))
	form.Set("text", text)
	if strings.TrimSpace(parseMode) != "" {
		form.Set("parse_mode", parseMode)
	}
	if replyMarkup != nil {
		payload, err := json.Marshal(replyMarkup)
		if err != nil {
			return err
		}
		form.Set("reply_markup", string(payload))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token),
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
		return fmt.Errorf("telegram editMessageText status %d: %s", response.StatusCode, string(body))
	}

	return nil
}

func answerCallbackQuery(ctx context.Context, client *http.Client, token, callbackQueryID, text string) error {
	form := url.Values{}
	form.Set("callback_query_id", callbackQueryID)
	if strings.TrimSpace(text) != "" {
		form.Set("text", text)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", token),
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
		return fmt.Errorf("telegram answerCallbackQuery status %d: %s", response.StatusCode, string(body))
	}

	return nil
}
