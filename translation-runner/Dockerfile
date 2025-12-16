FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git make

# Copy operator directory (for replace directive)
COPY operator ./operator

# Copy runner go mod files and update replace path
COPY translation-runner/go.mod translation-runner/go.sum ./
RUN sed -i 's|=> \./operator|=> ./operator|g' go.mod || true

# Download dependencies
RUN go mod download

# Copy runner source
COPY translation-runner/cmd ./cmd

# Build
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -o /translation-runner ./cmd/runner

FROM alpine:latest

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

COPY --from=builder /translation-runner /app/translation-runner

ENTRYPOINT ["/app/translation-runner"]

