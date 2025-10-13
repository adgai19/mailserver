# -------- Stage 1: Build --------
FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# -------- Stage 2: Run --------
FROM alpine:3.22

# Add certs (for HTTPS)
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/server .
COPY certs /certs

EXPOSE 80 443 25

CMD ["./server"]
