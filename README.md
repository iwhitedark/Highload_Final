# Руководство по развертыванию Highload Service

## Содержание

1. [Требования](#требования)
2. [Локальная разработка](#локальная-разработка)
3. [Сборка Docker образа](#сборка-docker-образа)
4. [Развертывание в Minikube](#развертывание-в-minikube)
5. [Развертывание в Killercoda](#развертывание-в-killercoda)
6. [Развертывание в Yandex.Cloud](#развертывание-в-yandexcloud)
7. [Настройка мониторинга](#настройка-мониторинга)
8. [Нагрузочное тестирование](#нагрузочное-тестирование)
9. [Troubleshooting](#troubleshooting)

---

## Требования

### Программное обеспечение

- Go 1.22+
- Docker 20.10+
- kubectl 1.28+
- Helm 3.12+
- Minikube 1.32+ / Kind 0.20+ (для локального развертывания)

### Ресурсы

- CPU: минимум 2 ядра
- RAM: минимум 4 GB
- Диск: 10 GB свободного места

---

## Локальная разработка

### 1. Клонирование репозитория

```bash
git clone <repository-url>
cd Highload_Ver_3.0
```

### 2. Установка зависимостей Go

```bash
go mod download
```

### 3. Запуск Redis локально

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### 4. Запуск сервиса

```bash
# Переменные окружения
export REDIS_ADDR=localhost:6379
export SERVER_ADDR=:8080

# Запуск
go run ./cmd/server
```

### 5. Проверка работоспособности

```bash
# Health check
curl http://localhost:8080/health

# Отправка метрики
curl -X POST http://localhost:8080/metrics \
  -H "Content-Type: application/json" \
  -d '{"timestamp":"2024-01-01T12:00:00Z","cpu":45.5,"rps":500}'

# Получение анализа
curl http://localhost:8080/analyze
```

---

## Сборка Docker образа

### Сборка

```bash
docker build -t highload-service:latest .
```

### Проверка размера образа

```bash
docker images highload-service:latest
# Ожидаемый размер: < 300 MB (alpine-based)
```

### Локальный запуск контейнера

```bash
docker run -d \
  --name highload-service \
  -p 8080:8080 \
  -e REDIS_ADDR=host.docker.internal:6379 \
  highload-service:latest
```

---

## Развертывание в Minikube

### 1. Запуск Minikube

```bash
minikube start --cpus=2 --memory=4g
```

### 2. Включение необходимых аддонов

```bash
minikube addons enable ingress
minikube addons enable metrics-server
```

### 3. Загрузка образа в Minikube

```bash
minikube image load highload-service:latest
```

### 4. Добавление Helm репозиториев

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```

### 5. Развертывание Redis

```bash
kubectl create namespace highload

helm install redis bitnami/redis \
  -n highload \
  -f k8s/helm/redis-values.yaml
```

### 6. Развертывание приложения

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/ingress.yaml
```

### 7. Проверка статуса

```bash
kubectl get pods -n highload
kubectl get svc -n highload
kubectl get hpa -n highload
```

### 8. Доступ к сервису

```bash
# Port forward
kubectl port-forward svc/highload-service -n highload 8080:80

# Или через Minikube service
minikube service highload-service -n highload
```

---

## Развертывание в Killercoda

### 1. Открытие Kubernetes Playground

Перейдите на https://killercoda.com/playgrounds/scenario/kubernetes

### 2. Установка Helm

```bash
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### 3. Создание Dockerfile и сборка

```bash
# Создайте структуру проекта
mkdir -p highload-service/cmd/server highload-service/internal/{analytics,cache,handlers,metrics,models}

# Скопируйте файлы проекта (или клонируйте репозиторий)
# ...

# Сборка образа
cd highload-service
docker build -t highload-service:latest .
```

### 4. Загрузка в Kind/встроенный кластер

```bash
kind load docker-image highload-service:latest
```

### 5. Развертывание

```bash
kubectl apply -f k8s/
```

---

## Развертывание в Yandex.Cloud

### 1. Установка Yandex Cloud CLI

```bash
curl -sSL https://storage.yandexcloud.net/yandexcloud-yc/install.sh | bash
```

### 2. Авторизация

```bash
yc init
```

### 3. Создание Kubernetes кластера

```bash
# Создание сети
yc vpc network create --name highload-network

# Создание подсети
yc vpc subnet create \
  --name highload-subnet \
  --network-name highload-network \
  --zone ru-central1-a \
  --range 10.1.0.0/16

# Создание кластера Kubernetes
yc managed-kubernetes cluster create \
  --name highload-cluster \
  --network-name highload-network \
  --zone ru-central1-a \
  --public-ip \
  --release-channel regular \
  --version 1.28

# Создание node group
yc managed-kubernetes node-group create \
  --name highload-nodes \
  --cluster-name highload-cluster \
  --cores 2 \
  --memory 4 \
  --core-fraction 100 \
  --disk-size 30 \
  --fixed-size 2
```

### 4. Настройка kubectl

```bash
yc managed-kubernetes cluster get-credentials highload-cluster --external
```

### 5. Загрузка образа в Container Registry

```bash
# Создание реестра
yc container registry create --name highload-registry

# Авторизация в реестре
yc container registry configure-docker

# Тегирование и push
docker tag highload-service:latest cr.yandex/<registry-id>/highload-service:latest
docker push cr.yandex/<registry-id>/highload-service:latest
```

### 6. Обновление манифестов

Обновите `k8s/deployment.yaml`:

```yaml
image: cr.yandex/<registry-id>/highload-service:latest
imagePullPolicy: Always
```

### 7. Развертывание

```bash
kubectl apply -f k8s/
```

---

## Настройка мониторинга

### 1. Развертывание Prometheus Stack

```bash
kubectl create namespace monitoring

helm install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring \
  -f k8s/helm/prometheus-values.yaml
```

### 2. Доступ к Grafana

```bash
kubectl port-forward svc/prometheus-grafana -n monitoring 3000:80
```

Откройте http://localhost:3000
- Логин: `admin`
- Пароль: `admin`

### 3. Импорт дашборда

1. В Grafana: Dashboards → Import
2. Загрузите файл `k8s/grafana-dashboard.json`

### 4. Доступ к Prometheus

```bash
kubectl port-forward svc/prometheus-kube-prometheus-prometheus -n monitoring 9090:9090
```

### 5. Проверка метрик

Откройте http://localhost:9090 и выполните запросы:

```promql
# RPS
rate(highload_requests_total[1m])

# Latency P99
histogram_quantile(0.99, sum(rate(highload_request_duration_seconds_bucket[5m])) by (le))

# Anomaly rate
increase(highload_anomalies_detected_total[1m])
```

---

## Нагрузочное тестирование

### Использование Apache Bench

```bash
# Создание файла с данными
echo '{"timestamp":"2024-01-01T12:00:00Z","cpu":50,"rps":500}' > /tmp/metric.json

# Тест на 1000 RPS (10000 запросов, 100 параллельных)
ab -n 10000 -c 100 -T "application/json" -p /tmp/metric.json http://localhost:8080/metrics
```

### Использование Locust

```bash
# Установка
pip install locust

# Запуск
cd scripts
locust -f locustfile.py --host=http://localhost:8080 --users=500 --spawn-rate=50 --run-time=5m
```

### Использование скрипта

```bash
chmod +x scripts/load-test.sh
./scripts/load-test.sh http://localhost:8080 300 500 locust
```

### Ожидаемые результаты

| Метрика | Ожидание |
|---------|----------|
| RPS | > 1000 |
| P99 Latency | < 50 ms |
| Error rate | < 1% |
| HPA scaling | 2 → 4 реплики при нагрузке |

---

## Troubleshooting

### Проблема: Pods в статусе Pending

```bash
# Проверка событий
kubectl describe pod <pod-name> -n highload

# Возможные причины:
# - Недостаточно ресурсов
# - Проблемы с PVC
```

### Проблема: Redis connection refused

```bash
# Проверка Redis
kubectl get pods -n highload -l app.kubernetes.io/name=redis

# Проверка секретов
kubectl get secret redis -n highload -o yaml
```

### Проблема: HPA не масштабирует

```bash
# Проверка metrics-server
kubectl get deployment metrics-server -n kube-system

# Проверка метрик
kubectl top pods -n highload
```

### Проблема: Ingress не работает

```bash
# Проверка ingress controller
kubectl get pods -n ingress-nginx

# Проверка ingress
kubectl describe ingress highload-ingress -n highload
```

### Логи сервиса

```bash
kubectl logs -f deployment/highload-service -n highload
```

### Профилирование

```bash
# Port forward для pprof
kubectl port-forward svc/highload-service -n highload 8080:80

# CPU profile
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:8080/debug/pprof/heap
```

---

## Команды быстрого старта

```bash
# Полное развертывание в Minikube
chmod +x scripts/deploy.sh
./scripts/deploy.sh minikube

# Только приложение
./scripts/deploy.sh app

# Нагрузочный тест
./scripts/load-test.sh http://$(minikube ip):$(kubectl get svc highload-service -n highload -o jsonpath='{.spec.ports[0].nodePort}') 300 500
```
