FROM golang:1.21 AS builder

WORKDIR /app

COPY . .
RUN rm -f go.sum
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o compounder ./cmd/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/compounder .

CMD ["./compounder"]
