# syntax=docker/dockerfile:1

# --- Build stage ---
FROM golang:1.22 as builder
WORKDIR /app

# Cache deps
COPY go.mod ./
RUN go mod download

# Copy source
COPY . .

# Build API
WORKDIR /app/services/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api

# --- Runtime stage ---
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /
COPY --from=builder /out/api /api

ENV API_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]

