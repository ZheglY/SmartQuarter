FROM golang:1.26-bookworm AS build
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/app /app
USER 65532:65532
ENV GRPC_ADDR=:8082 HTTP_ADDR=:8083
EXPOSE 8082 8083
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 CMD ["/app", "healthcheck"]
ENTRYPOINT ["/app"]
