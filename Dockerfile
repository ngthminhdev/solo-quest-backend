# ─── Build stage ─────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app/server ./cmd/api/main.go

# ─── Run stage ───────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Ho_Chi_Minh /etc/localtime && \
    echo "Asia/Ho_Chi_Minh" > /etc/timezone

ENV TZ=Asia/Ho_Chi_Minh

WORKDIR /app

COPY --from=builder /app/server ./server
COPY --from=builder /app/migrations ./migrations

EXPOSE 9000

ENTRYPOINT ["./server"]
