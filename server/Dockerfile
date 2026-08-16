# syntax=docker/dockerfile:1

FROM node:20-bookworm-slim AS web

WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm config set registry https://registry.npmmirror.com \
    && npm ci

COPY web/ ./
RUN npm run build


FROM golang:1.26.4-bookworm AS build

WORKDIR /src

ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOPROXY=https://goproxy.cn,direct
ARG BUILD_TAGS=fuse

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/internal/api/web /src/internal/api/web

RUN go build -tags "${BUILD_TAGS}" -trimpath -ldflags="-s -w" -o /out/xmedia ./cmd/xmedia


FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata fuse3 \
    && sed -i 's/^#user_allow_other/user_allow_other/' /etc/fuse.conf 2>/dev/null || true \
    && grep -q '^user_allow_other' /etc/fuse.conf || echo user_allow_other >> /etc/fuse.conf \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN mkdir -p /app/data/log /app/strm /app/mounts

COPY --from=build /out/xmedia /app/xmedia

ENV XMEDIA_DATA_DIR=/app/data \
    XMEDIA_STRM_DIR=/app/strm \
    XMEDIA_LISTEN=:38088 \
    XMEDIA_LOG_LEVEL=info \
    TZ=Asia/Shanghai

EXPOSE 38088

VOLUME ["/app/data", "/app/strm", "/app/mounts"]

ENTRYPOINT ["/app/xmedia"]
