# Multi-stage Dockerfile for Go API (scaffold)

FROM golang:1.22-alpine AS base
WORKDIR /app

# Install build deps
RUN apk add --no-cache git ca-certificates tzdata && update-ca-certificates

## Pre-cache modules
COPY go.mod ./
RUN go mod download || true

## Copy source
COPY . .

## Build API binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./cmd/api

FROM alpine:3.19 AS runtime
RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates
WORKDIR /srv

# Copy binary
COPY --from=base /out/app /usr/local/bin/app

ENV PORT=8080
EXPOSE 8080

## Run API
CMD ["/usr/local/bin/app"]
