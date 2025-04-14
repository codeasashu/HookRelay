FROM golang:1.21 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o webhook ./cmd/cmd.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/webhook .

COPY config.toml .

EXPOSE 8081

# Run the server
CMD ["./webhook", "server", "-c", "config.toml", "-l"]
