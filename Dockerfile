FROM golang:1.25-alpine AS build

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/notifier-telegram-bot ./cmd/notifier-telegram-bot

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && \
    adduser -S -G app app

COPY --from=build /bin/notifier-telegram-bot /usr/local/bin/notifier-telegram-bot
USER app
ENTRYPOINT ["notifier-telegram-bot"]
