# syntax=docker/dockerfile:1
# One Dockerfile, six images. Build arg MODULE_ID selects the content.
ARG MODULE_ID=linux-cli

FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /pod ./cmd/pod

# ── Runtime image ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache \
    bash curl git docker-cli \
    grep sed gawk coreutils \
    ca-certificates

ARG MODULE_ID
WORKDIR /app
COPY --from=builder /pod .
COPY modules/${MODULE_ID}/v1/ ./content/

ENV MODULE_ID=${MODULE_ID}
ENV CONTENT_ROOT=/app/content

EXPOSE 8080
CMD ["./pod"]
