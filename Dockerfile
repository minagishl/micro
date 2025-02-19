FROM golang:1.24 AS builder

WORKDIR /app

# Copy dependencies first to utilize the cache of Go Modules
COPY go.mod go.sum ./
RUN go mod tidy

# Copy the application code
COPY . .

# Build the Go app with CGO disabled for a fully static binary
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o micro .

# Lightweight image for execution environment
FROM debian:bullseye-slim
WORKDIR /app

# Copy only the binary
COPY --from=builder /app/micro /app/micro

# Run the binary
CMD ["/app/micro"]
