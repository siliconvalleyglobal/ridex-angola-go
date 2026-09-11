# ── syntax=docker/dockerfile:1
FROM golang:1.27.1-alpine AS base

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# ── Builder stage ──────────────────────────────────────────────────────
FROM base AS builder

RUN apk add --no-cache gcc musl-dev pkgconfig postgresql-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /bin/matching ./cmd/matching
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /bin/worker ./cmd/worker

# ── Dev stage (for docker-compose dev service) ────────────────────────
FROM base AS dev

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Keep containers alive; devs run `go run` inside
CMD ["sleep", "infinity"]

# ── Prod stage ─────────────────────────────────────────────────────────
FROM alpine:3.21 AS prod

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/matching /bin/matching
COPY --from=builder /bin/worker /bin/worker

EXPOSE 8080 9090

# Default: run the API server
CMD ["/bin/api"]
