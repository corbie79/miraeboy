# ── 1단계: 프론트엔드 빌드 ─────────────────────────────────────────────────────
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci --silent
COPY web/ ./
RUN npm run build

# ── 2단계: Go 빌드 ─────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS go-builder
WORKDIR /app

# 의존성만 먼저 캐시
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /app/web/dist ./web/dist

# S3 지원을 포함하려면: --build-arg BUILD_TAGS=s3
ARG BUILD_TAGS=""
RUN CGO_ENABLED=0 GOOS=linux \
    go build ${BUILD_TAGS:+-tags $BUILD_TAGS} \
    -ldflags="-s -w" \
    -o /miraeboy .

# ── 3단계: 실행 이미지 ──────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S miraeboy && \
    adduser -S -G miraeboy miraeboy

WORKDIR /etc/miraeboy
COPY --from=go-builder /miraeboy /usr/local/bin/miraeboy

# 기본 설정 파일 (볼륨 마운트로 덮어쓸 수 있음)
COPY config.yaml /etc/miraeboy/config.yaml

RUN mkdir -p /var/lib/miraeboy/data && \
    chown -R miraeboy:miraeboy /var/lib/miraeboy && \
    chown -R miraeboy:miraeboy /etc/miraeboy

USER miraeboy

VOLUME ["/etc/miraeboy", "/var/lib/miraeboy/data"]

EXPOSE 9300

ENTRYPOINT ["/usr/local/bin/miraeboy"]
