FROM node:24-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_API_BASE_URL=
ARG VITE_S3_ORIGINS=https://storage.yandexcloud.net
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL VITE_S3_ORIGINS=$VITE_S3_ORIGINS
RUN npm run build

FROM nginx:1.28-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
