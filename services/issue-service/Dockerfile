FROM golang:1.26-bookworm AS build
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0
# Service directory is the entire build context, including its own go.mod/go.sum.
COPY . .
RUN go mod download && go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/app /app
USER 65532:65532
ENV HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app"]

