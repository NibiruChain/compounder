FROM golang:1.21 AS builder

ENV GOOS=linux
# ENV GOARCH=arm64

WORKDIR /app
COPY . .
RUN rm -f go.sum
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o compounder .
RUN chmod +x compounder

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/compounder .
CMD ["./compounder"]
