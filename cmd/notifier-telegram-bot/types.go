package main

import "strings"

type telegramUpdateResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID      int64                  `json:"update_id"`
	Message       *telegramMessage       `json:"message"`
	CallbackQuery *telegramCallbackQuery `json:"callback_query"`
}

type telegramMessage struct {
	MessageID int64         `json:"message_id"`
	Text      string        `json:"text"`
	Chat      telegramChat  `json:"chat"`
	From      *telegramUser `json:"from"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	Username *string `json:"username"`
}

type telegramCallbackQuery struct {
	ID      string           `json:"id"`
	Data    string           `json:"data"`
	Message *telegramMessage `json:"message"`
}

type telegramInlineKeyboardMarkup struct {
	InlineKeyboard [][]telegramInlineKeyboardButton `json:"inline_keyboard"`
}

type telegramInlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

type apiUser struct {
	ID               string  `json:"id"`
	AccountID        string  `json:"account_id,omitempty"`
	UserID           string  `json:"user_id,omitempty"`
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

const listPageSize = 8

func (user apiUser) accountID() string {
	for _, value := range []string{user.ID, user.AccountID, user.UserID} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return "неизвестен"
}
