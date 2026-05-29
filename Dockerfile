FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /dokvol ./cmd/server/

FROM alpine:3.21

RUN apk add --no-cache \
    rsync \
    ca-certificates \
    tzdata

COPY --from=builder /dokvol /usr/local/bin/dokvol
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
CMD ["dokvol"]
