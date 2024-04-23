FROM golang:1.21 AS builder

WORKDIR /app
COPY . .
RUN go mod tidy
ENV CGO_ENABLED=0 GOOS=linux GOARCH=arm64
RUN go build -o compounder ./cmd/main.go
RUN chmod +x compounder

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/compounder .
CMD ["./compounder"]
