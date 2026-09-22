# Stage 1: Сборка бинарного файла
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Установка системных сертификатов для безопасных соединений
RUN apk add --no-cache ca-certificates

# Кэширование зависимостей Go
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/app /app
USER 65532:65532
EXPOSE 50051 8081
HEALTHCHECK --interval=5s --timeout=4s --start-period=15s CMD ["/app", "healthcheck"]
ENTRYPOINT ["/app"]
