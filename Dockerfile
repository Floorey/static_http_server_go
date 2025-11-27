# ---------- Build Stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# go.mod kopieren (go.sum existiert NICHT – also NICHT kopieren)
COPY go.mod ./

# Dependencies laden (Standardlib → kein Download nötig, aber harmless)
RUN go mod download || true

# Restliches Projekt
COPY . .

# Go-Binary statisch bauen
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o ratskeller-site ./cmd/server

# ---------- Runtime Stage ----------
FROM alpine:3.20

# Non-root User für Sicherheit
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

# Binary + statische Dateien aus Build-Stage holen
COPY --from=builder /app/ratskeller-site .
COPY --from=builder /app/public ./public

ENV PORT=8080
EXPOSE 8080

USER app

CMD ["./ratskeller-site"]
