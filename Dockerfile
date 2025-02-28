FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o breeze ./cmd/server

FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/breeze .

RUN mkdir -p ./data

EXPOSE 8080

CMD ["./breeze"]
