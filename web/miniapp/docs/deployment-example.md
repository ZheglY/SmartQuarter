# Пример подключения к общей инфраструктуре

Это пример для команды интеграции, не применённое изменение deploy/.

```yaml
services:
  miniapp:
    build:
      context: ../../web/miniapp
      args:
        VITE_API_BASE_URL: ""
        VITE_S3_ORIGINS: "https://storage.yandexcloud.net"
    expose:
      - "8080"
    restart: unless-stopped
```

HTTPS ingress / nginx после настройки сертификата:

```nginx
location /api/v1/ {
    proxy_pass http://max-gateway:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    # Origin и Cookie передаются как пришли; не подставлять Origin вместо клиента.
}
location / {
    proxy_pass http://miniapp:8080;
    proxy_set_header Host $host;
}
```

Пути API сохраняются полностью; `proxy_pass` без завершающего `/`. Hostnames соответствуют именам сервисов конкретного deployment, их надо согласовать. Настройте Gateway `TRUSTED_ORIGINS=https://app.example.com`, production cookie, публичный S3 endpoint и CORS. Зарегистрируйте `https://app.example.com` как Mini App URL бота. TLS, webhook routing и секреты остаются у общего deployment, в образ frontend не включаются.

Healthcheck frontend `/healthz` означает только доступность статического сервера. Здоровье бизнес-сервисов проверяется отдельно `/readyz` Gateway. При текущем identity.Unavailable оно 503, даже если nginx frontend работает.
