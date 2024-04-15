.PHONY: build test docker-build docker-push deploy

build:
	go build -o compounder ./cmd/main.go

test:
	go test -v ./...

docker-build:
	docker build -t compounder:latest .


docker-run:
	docker run --rm compounder:latest
