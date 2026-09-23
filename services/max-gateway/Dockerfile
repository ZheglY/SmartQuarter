FROM golang:1.27-bookworm AS build
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
ENV HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s CMD ["/app", "healthcheck"]
ENTRYPOINT ["/app"]
