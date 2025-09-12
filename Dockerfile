# Stage 1: Build the application
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a fully static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./dist/shopit_api ./cmd/api

# Stage 2: Create the final, minimal, and secure image
FROM gcr.io/distroless/static

WORKDIR /app

# Copy the static binary
COPY --from=builder /app/dist/shopit_api .

# Copy the configuration directory.
COPY --from=builder /app/config/ ./config/

EXPOSE 8080

CMD ["/app/shopit_api"]