# Этап сборки
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Копируем модули конкретно этого сервиса (они теперь в корне контекста)
COPY go.mod go.sum* ./
RUN go mod download

# Копируем весь остальной исходный код этого сервиса
COPY . .

# Собираем приложение (путь изменился, так как мы уже внутри папки сервиса)
RUN CGO_ENABLED=0 GOOS=linux go build -o /community-service ./cmd/app

# Финальный образ
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /community-service .

EXPOSE 8084 9090

CMD ["./community-service"]
