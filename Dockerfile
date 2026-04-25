FROM golang:1.25-alpine AS build

WORKDIR /app
COPY . .
RUN go build -o /bin/notifier-telegram-bot ./cmd/notifier-telegram-bot

FROM alpine:3.21

WORKDIR /app
COPY --from=build /bin/notifier-telegram-bot /usr/local/bin/notifier-telegram-bot
ENTRYPOINT ["notifier-telegram-bot"]
