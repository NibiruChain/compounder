DOCKER_REGISTRY ?= your-registry
IMAGE_NAME ?= compounder
IMAGE_TAG ?= latest

.PHONY: build test docker-build docker-push deploy

build:
	go build -o compounder ./cmd/main.go

test:
	go test -v ./...

docker-build:
	docker build -t $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) .

docker-push:
	docker push $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

deploy: docker-build docker-push
	kubectl apply -f kubernetes/deployment.yaml
	kubectl apply -f kubernetes/cronjob.yaml