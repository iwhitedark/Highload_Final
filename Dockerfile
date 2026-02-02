# Multi-stage build for minimal image size (< 300 MB)
# Stage 1: Build
FROM golang:1.22-alpine AS builder

# Установка зависимостей для сборки
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum* ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник с оптимизациями
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=1.0.0" \
    -o /highload-service \
    ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19

# Устанавливаем ca-certificates для HTTPS и tzdata для временных зон
RUN apk --no-cache add ca-certificates tzdata

# Создаем непривилегированного пользователя
RUN adduser -D -g '' appuser

WORKDIR /app

# Копируем бинарник из builder stage
COPY --from=builder /highload-service .

# Устанавливаем владельца
RUN chown -R appuser:appuser /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Запускаем сервис
ENTRYPOINT ["./highload-service"]
