# notifier-telegram-bot

Telegram bot service for quick reminder and note actions.

Сервис Telegram-бота для быстрых действий с заметками и напоминаниями.

## Current Commands

- `/start CODE` - link Telegram to a Notifier account
- `CODE` - link Telegram with a plain code message
- `/me` - show the linked Notifier account
- `/list` - show recent items from the linked account

- `/start CODE` - привязать Telegram к аккаунту Notifier
- `CODE` - привязать Telegram обычным сообщением с кодом
- `/me` - показать привязанный аккаунт Notifier
- `/list` - показать последние записи из привязанного аккаунта

## Current Behavior

- The bot polls Telegram with `getUpdates`
- The bot clears old webhooks on startup
- The bot links Telegram chats through the API
- The bot fetches recent items through the API
- The bot sends pending Telegram reminder deliveries prepared by the API

- Бот опрашивает Telegram через `getUpdates`
- Бот удаляет старые webhook при старте
- Бот привязывает Telegram-чаты через API
- Бот получает последние записи через API
- Бот отправляет ожидающие доставки напоминаний в Telegram, которые подготовил API

## Integration Rules

- The bot authenticates against the API
- Telegram chat links are stored by the API
- Reminder delivery jobs are created by the API and sent by the bot
- Business logic stays in the API so web and desktop behave the same

- Бот аутентифицируется перед API
- Привязки Telegram-чатов хранятся в API
- Задания на доставку напоминаний создает API, а отправляет их бот
- Бизнес-логика остается в API, чтобы web и desktop работали одинаково
