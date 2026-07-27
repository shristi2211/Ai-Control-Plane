# ── Build stage ──
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ai-proxy .

# ── Runtime stage ──
FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /app/ai-proxy .

EXPOSE 8080
CMD ["./ai-proxy"]
