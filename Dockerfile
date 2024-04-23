FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o compounder ./cmd/main.go

FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /app/compounder .
CMD ["/compounder"]
