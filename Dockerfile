# ── Frontend builder ──────────────────────────────────────────
FROM oven/bun:1 AS frontend
WORKDIR /app
COPY interface/bun.lock interface/package.json ./
RUN bun install --frozen-lockfile
COPY interface/ .
RUN bun run build

# ── Backend builder ───────────────────────────────────────────
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /usr/local/bin/dokvol ./cmd/dokvol/

# ── Final ─────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache \
    rsync \
    ca-certificates \
    tzdata \
    docker-cli

COPY --from=frontend /app/build /usr/local/share/dokvol/static
COPY --from=backend /usr/local/bin/dokvol /usr/local/bin/dokvol
COPY --from=backend /src/migrations /usr/local/share/dokvol/migrations
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

WORKDIR /usr/local/share/dokvol

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
CMD ["dokvol", "server"]
