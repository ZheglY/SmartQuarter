# Build the same upstream releases from source: their public registry images are unavailable.
FROM golang:1.26.5-bookworm AS minio-build
RUN CGO_ENABLED=0 go install -p=2 github.com/minio/minio@07c3a429bfed433e49018cb0f78a52145d4bedeb \
    && mkdir /licenses \
    && cp /go/pkg/mod/github.com/minio/minio@*/LICENSE /licenses/LICENSE

FROM golang:1.26.5-bookworm AS mc-build
RUN CGO_ENABLED=0 go install -p=2 github.com/minio/mc@7394ce0dd2a80935aded936b09fa12cbb3cb8096 \
    && mkdir /licenses \
    && cp /go/pkg/mod/github.com/minio/mc@*/LICENSE /licenses/LICENSE

FROM debian:bookworm-slim AS minio
COPY --from=minio-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=minio-build /licenses /usr/share/licenses/minio
COPY --from=minio-build /go/bin/minio /usr/local/bin/minio
ENTRYPOINT ["minio"]
CMD ["server", "/data"]

FROM debian:bookworm-slim AS mc
COPY --from=mc-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=mc-build /licenses /usr/share/licenses/mc
COPY --from=mc-build /go/bin/mc /usr/local/bin/mc
ENTRYPOINT ["mc"]
