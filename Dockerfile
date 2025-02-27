FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o gust-api ./cmd/server

FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/gust-api .

# Create data directory
RUN mkdir -p ./data

EXPOSE 8080

CMD ["./gust-api"]
