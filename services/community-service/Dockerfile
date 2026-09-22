# Этап сборки
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# ВАЖНО: здесь должно быть /community-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /community-service ./cmd/app

# Финальный образ
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /community-service .

EXPOSE 8084 9090

CMD ["./community-service"]
