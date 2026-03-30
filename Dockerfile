# syntax=docker/dockerfile:1

# ── Stage 1: Build ──────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install deps first (cache-friendly)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./main.go

# ── Stage 2: Runtime ─────────────────────────────────────────────
FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

COPY --from=builder /app/server .

EXPOSE 3457

CMD ["./server"]
