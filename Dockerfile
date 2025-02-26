FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o weather-api .

FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/weather-api .
COPY --from=builder /app/.env* .

EXPOSE 8080

CMD ["./weather-api"]
