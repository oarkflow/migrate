# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26.2-alpine AS builder

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrator ./cmd/migrator

# ---- Final stage ----
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/migrator /app/migrator

ENTRYPOINT ["/app/migrator"]
