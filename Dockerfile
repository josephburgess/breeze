FROM golang:1.20 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

RUN go build -o breeze ./cmd/server

# Use a smaller base image for the final container
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/breeze .

EXPOSE 8080

CMD ["./breeze"]
