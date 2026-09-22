# Stage 1: Сборка бинарного файла
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Установка системных сертификатов для безопасных соединений
RUN apk add --no-cache ca-certificates

# Кэширование зависимостей Go
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Статическая компиляция без CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/identity-service ./cmd/app/main.go

# Stage 2: Минимальный финальный образ
FROM alpine:3.20

WORKDIR /app

# Непривилегированный системный пользователь для безопасности
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/identity-service /app/identity-service

# Проброс портов: 50051 (gRPC), 8081 (Technical HTTP)
EXPOSE 50051 8081

USER appuser

ENTRYPOINT ["/app/identity-service"]
