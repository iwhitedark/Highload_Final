# Makefile for Highload Service
# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Binary names
BINARY_NAME=highload-service
BINARY_UNIX=$(BINARY_NAME)_unix

# Docker
DOCKER_IMAGE=highload-service
DOCKER_TAG=latest

# Kubernetes
NAMESPACE=highload

.PHONY: all build clean test coverage docker-build docker-push deploy help

all: test build

## Build
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/server

build-linux:
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o $(BINARY_UNIX) ./cmd/server

## Test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-short:
	@echo "Running short tests..."
	$(GOTEST) -short ./...

coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./internal/analytics/

## Code quality
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

lint:
	@echo "Running linter..."
	golangci-lint run

vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## Dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

## Clean
clean:
	@echo "Cleaning..."
	$(GOCMD) clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f coverage.out coverage.html

## Docker
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Image size:"
	@docker images $(DOCKER_IMAGE):$(DOCKER_TAG) --format "{{.Size}}"

docker-run:
	@echo "Running Docker container..."
	docker run -d --name $(BINARY_NAME) -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-stop:
	@echo "Stopping Docker container..."
	docker stop $(BINARY_NAME) || true
	docker rm $(BINARY_NAME) || true

docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

## Kubernetes / Minikube
minikube-start:
	@echo "Starting Minikube..."
	minikube start --cpus=2 --memory=4g
	minikube addons enable ingress
	minikube addons enable metrics-server

minikube-load:
	@echo "Loading image into Minikube..."
	minikube image load $(DOCKER_IMAGE):$(DOCKER_TAG)

## Deploy
deploy-redis:
	@echo "Deploying Redis..."
	helm repo add bitnami https://charts.bitnami.com/bitnami || true
	helm repo update
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install redis bitnami/redis -n $(NAMESPACE) -f k8s/helm/redis-values.yaml

deploy-monitoring:
	@echo "Deploying Prometheus and Grafana..."
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts || true
	helm repo update
	kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install prometheus prometheus-community/kube-prometheus-stack -n monitoring -f k8s/helm/prometheus-values.yaml

deploy-app:
	@echo "Deploying application..."
	kubectl apply -f k8s/namespace.yaml
	kubectl apply -f k8s/configmap.yaml
	kubectl apply -f k8s/deployment.yaml
	kubectl apply -f k8s/service.yaml
	kubectl apply -f k8s/hpa.yaml
	kubectl apply -f k8s/ingress.yaml
	@echo "Waiting for deployment..."
	kubectl rollout status deployment/highload-service -n $(NAMESPACE) --timeout=120s

deploy: deploy-redis deploy-app
	@echo "Deployment complete!"

deploy-all: deploy-redis deploy-monitoring deploy-app
	@echo "Full deployment complete!"

## Status
status:
	@echo "=== Pods ==="
	kubectl get pods -n $(NAMESPACE)
	@echo "\n=== Services ==="
	kubectl get svc -n $(NAMESPACE)
	@echo "\n=== HPA ==="
	kubectl get hpa -n $(NAMESPACE)
	@echo "\n=== Ingress ==="
	kubectl get ingress -n $(NAMESPACE)

logs:
	kubectl logs -f deployment/highload-service -n $(NAMESPACE)

## Port forwarding
port-forward:
	@echo "Port forwarding to service..."
	kubectl port-forward svc/highload-service -n $(NAMESPACE) 8080:80

grafana:
	@echo "Port forwarding to Grafana..."
	kubectl port-forward svc/prometheus-grafana -n monitoring 3000:80

prometheus:
	@echo "Port forwarding to Prometheus..."
	kubectl port-forward svc/prometheus-kube-prometheus-prometheus -n monitoring 9090:9090

## Load testing
load-test:
	@echo "Running load test..."
	chmod +x scripts/load-test.sh
	./scripts/load-test.sh http://localhost:8080 300 500 locust

load-test-ab:
	@echo "Running Apache Bench test..."
	@echo '{"timestamp":"2024-01-01T12:00:00Z","cpu":50,"rps":500}' > /tmp/metric.json
	ab -n 10000 -c 100 -T "application/json" -p /tmp/metric.json http://localhost:8080/metrics

## Local development
run:
	@echo "Running locally..."
	REDIS_ADDR=localhost:6379 $(GOCMD) run ./cmd/server

redis-local:
	@echo "Starting local Redis..."
	docker run -d --name redis-local -p 6379:6379 redis:7-alpine || docker start redis-local

## Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  test           - Run tests"
	@echo "  coverage       - Run tests with coverage"
	@echo "  benchmark      - Run benchmarks"
	@echo "  docker-build   - Build Docker image"
	@echo "  deploy         - Deploy to Kubernetes (Redis + App)"
	@echo "  deploy-all     - Deploy everything (Redis + Monitoring + App)"
	@echo "  status         - Show deployment status"
	@echo "  logs           - Show application logs"
	@echo "  port-forward   - Port forward to service"
	@echo "  load-test      - Run load test"
	@echo "  run            - Run locally"
	@echo "  help           - Show this help"
