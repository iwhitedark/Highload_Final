#!/bin/bash
# Deployment script for Highload Service
# Supports Minikube, Kind, and cloud Kubernetes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Highload Service Deployment Script${NC}"
echo -e "${GREEN}========================================${NC}"

# Check prerequisites
check_prerequisites() {
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"

    command -v kubectl >/dev/null 2>&1 || { echo -e "${RED}kubectl is required but not installed.${NC}" >&2; exit 1; }
    command -v docker >/dev/null 2>&1 || { echo -e "${RED}docker is required but not installed.${NC}" >&2; exit 1; }
    command -v helm >/dev/null 2>&1 || { echo -e "${RED}helm is required but not installed.${NC}" >&2; exit 1; }

    echo -e "${GREEN}All prerequisites are installed.${NC}"
}

# Build Docker image
build_image() {
    echo -e "\n${YELLOW}Building Docker image...${NC}"

    cd "$(dirname "$0")/.."

    docker build -t highload-service:latest .

    # Get image size
    SIZE=$(docker images highload-service:latest --format "{{.Size}}")
    echo -e "${GREEN}Image built successfully. Size: ${SIZE}${NC}"
}

# Setup Minikube
setup_minikube() {
    echo -e "\n${YELLOW}Setting up Minikube...${NC}"

    # Check if Minikube is running
    if ! minikube status >/dev/null 2>&1; then
        echo "Starting Minikube..."
        minikube start --cpus=2 --memory=4g
    fi

    # Enable addons
    minikube addons enable ingress
    minikube addons enable metrics-server

    # Load image into Minikube
    echo "Loading image into Minikube..."
    minikube image load highload-service:latest

    echo -e "${GREEN}Minikube setup complete.${NC}"
}

# Deploy Redis
deploy_redis() {
    echo -e "\n${YELLOW}Deploying Redis...${NC}"

    # Add Bitnami repo
    helm repo add bitnami https://charts.bitnami.com/bitnami 2>/dev/null || true
    helm repo update

    # Create namespace if not exists
    kubectl create namespace highload --dry-run=client -o yaml | kubectl apply -f -

    # Install Redis
    helm upgrade --install redis bitnami/redis \
        -n highload \
        -f "$(dirname "$0")/../k8s/helm/redis-values.yaml" \
        --wait

    echo -e "${GREEN}Redis deployed successfully.${NC}"
}

# Deploy Prometheus and Grafana
deploy_monitoring() {
    echo -e "\n${YELLOW}Deploying Prometheus and Grafana...${NC}"

    # Add Prometheus community repo
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
    helm repo update

    # Create monitoring namespace
    kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

    # Install Prometheus stack
    helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
        -n monitoring \
        -f "$(dirname "$0")/../k8s/helm/prometheus-values.yaml" \
        --wait

    echo -e "${GREEN}Monitoring stack deployed successfully.${NC}"
}

# Deploy application
deploy_app() {
    echo -e "\n${YELLOW}Deploying Highload Service...${NC}"

    cd "$(dirname "$0")/.."

    # Apply Kubernetes manifests
    kubectl apply -f k8s/namespace.yaml
    kubectl apply -f k8s/configmap.yaml
    kubectl apply -f k8s/deployment.yaml
    kubectl apply -f k8s/service.yaml
    kubectl apply -f k8s/hpa.yaml
    kubectl apply -f k8s/ingress.yaml

    # Wait for deployment
    echo "Waiting for deployment to be ready..."
    kubectl rollout status deployment/highload-service -n highload --timeout=120s

    echo -e "${GREEN}Highload Service deployed successfully.${NC}"
}

# Show status
show_status() {
    echo -e "\n${YELLOW}Deployment Status:${NC}"

    echo -e "\n${GREEN}Pods:${NC}"
    kubectl get pods -n highload

    echo -e "\n${GREEN}Services:${NC}"
    kubectl get svc -n highload

    echo -e "\n${GREEN}HPA:${NC}"
    kubectl get hpa -n highload

    echo -e "\n${GREEN}Ingress:${NC}"
    kubectl get ingress -n highload
}

# Get access URLs
show_urls() {
    echo -e "\n${YELLOW}Access URLs:${NC}"

    # Minikube specific
    if command -v minikube >/dev/null 2>&1 && minikube status >/dev/null 2>&1; then
        MINIKUBE_IP=$(minikube ip)
        echo -e "Service URL: http://${MINIKUBE_IP}:$(kubectl get svc highload-service -n highload -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "80")"
        echo -e "Or use: minikube service highload-service -n highload"
    fi

    echo -e "\nPort forward for local access:"
    echo -e "  kubectl port-forward svc/highload-service -n highload 8080:80"
    echo -e "\nGrafana:"
    echo -e "  kubectl port-forward svc/prometheus-grafana -n monitoring 3000:80"
    echo -e "  Login: admin / admin"
    echo -e "\nPrometheus:"
    echo -e "  kubectl port-forward svc/prometheus-kube-prometheus-prometheus -n monitoring 9090:9090"
}

# Main
main() {
    check_prerequisites
    build_image

    # Parse arguments
    case "${1:-all}" in
        "minikube")
            setup_minikube
            deploy_redis
            deploy_monitoring
            deploy_app
            ;;
        "app")
            deploy_app
            ;;
        "redis")
            deploy_redis
            ;;
        "monitoring")
            deploy_monitoring
            ;;
        "all"|*)
            if command -v minikube >/dev/null 2>&1; then
                setup_minikube
            fi
            deploy_redis
            deploy_monitoring
            deploy_app
            ;;
    esac

    show_status
    show_urls

    echo -e "\n${GREEN}========================================${NC}"
    echo -e "${GREEN}Deployment Complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
}

main "$@"
