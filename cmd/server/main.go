// Package main запускает высоконагруженный сервис для обработки потоковых данных
// Сервис реализует:
// - HTTP API для приема метрик от IoT-устройств
// - Rolling average для сглаживания нагрузки (окно 50 событий)
// - Z-score детекцию аномалий (threshold > 2σ)
// - Кэширование в Redis
// - Экспорт метрик в Prometheus
package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"highload-service/internal/analytics"
	"highload-service/internal/cache"
	"highload-service/internal/handlers"
	"highload-service/internal/metrics"
)

// Config содержит конфигурацию сервиса
type Config struct {
	ServerAddr     string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	WorkerCount    int
	BufferSize     int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
}

func main() {
	log.Println("Starting Highload Service...")
	log.Printf("Go version: %s", runtime.Version())
	log.Printf("NumCPU: %d", runtime.NumCPU())

	// Загружаем конфигурацию
	cfg := loadConfig()

	// Инициализируем анализатор метрик
	analyzer := analytics.NewAnalyzer(cfg.BufferSize)
	analyzer.Start(cfg.WorkerCount)
	log.Printf("Analytics engine started with %d workers", cfg.WorkerCount)

	// Инициализируем Redis кэш
	var redisCache *cache.RedisCache
	var err error

	// Пробуем подключиться к Redis с повторами
	for i := 0; i < 5; i++ {
		redisCache, err = cache.NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		if err == nil {
			log.Printf("Connected to Redis at %s", cfg.RedisAddr)
			break
		}
		log.Printf("Redis connection attempt %d failed: %v", i+1, err)
		if i < 4 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if err != nil {
		log.Printf("Warning: Failed to connect to Redis, running without cache: %v", err)
		redisCache = nil
	}

	// Создаем обработчики
	handler := handlers.NewHandler(analyzer, redisCache)

	// Настраиваем маршруты
	router := mux.NewRouter()

	// API эндпоинты
	router.HandleFunc("/metrics", handler.MetricsHandler).Methods("POST")
	router.HandleFunc("/metrics/batch", handler.BatchMetricsHandler).Methods("POST")
	router.HandleFunc("/metrics/latest", handler.LatestMetricsHandler).Methods("GET")
	router.HandleFunc("/analyze", handler.AnalyzeHandler).Methods("GET")
	router.HandleFunc("/health", handler.HealthHandler).Methods("GET")
	router.HandleFunc("/stats", handler.StatsHandler).Methods("GET")

	// Prometheus метрики
	router.Handle("/prometheus", promhttp.Handler())

	// pprof для профилирования
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)

	// Middleware для логирования и метрик
	router.Use(loggingMiddleware)
	router.Use(metricsMiddleware)

	// Создаем HTTP сервер с настройками таймаутов
	server := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Запускаем горутину для обновления метрик
	go updateMetricsLoop(analyzer)

	// Запускаем горутину для обработки результатов анализа
	go processAnalysisResults(analyzer, redisCache)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем сервер в горутине
	go func() {
		log.Printf("Server listening on %s", cfg.ServerAddr)
		log.Printf("Endpoints:")
		log.Printf("  POST /metrics       - Submit metric data")
		log.Printf("  POST /metrics/batch - Submit batch metrics")
		log.Printf("  GET  /metrics/latest- Get latest metrics")
		log.Printf("  GET  /analyze       - Get analysis statistics")
		log.Printf("  GET  /health        - Health check")
		log.Printf("  GET  /stats         - Service statistics")
		log.Printf("  GET  /prometheus    - Prometheus metrics")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Ожидаем сигнал завершения
	<-stop
	log.Println("Shutting down server...")

	// Контекст с таймаутом для завершения
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Останавливаем анализатор
	analyzer.Stop()

	// Закрываем Redis
	if redisCache != nil {
		redisCache.Close()
	}

	// Завершаем HTTP сервер
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// loadConfig загружает конфигурацию из переменных окружения
func loadConfig() Config {
	return Config{
		ServerAddr:     getEnv("SERVER_ADDR", ":8080"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		WorkerCount:    getEnvInt("WORKER_COUNT", runtime.NumCPU()),
		BufferSize:     getEnvInt("BUFFER_SIZE", 10000),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
	}
}

// getEnv получает переменную окружения с значением по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt получает целочисленную переменную окружения
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		_, err := time.ParseDuration(value)
		if err == nil {
			return defaultValue
		}
		n, _ := time.ParseDuration("0")
		result = int(n.Seconds())
		if result > 0 {
			return result
		}
	}
	return defaultValue
}

// loggingMiddleware логирует HTTP запросы
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// metricsMiddleware обновляет метрики для каждого запроса
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.CurrentRPS.Inc()
		next.ServeHTTP(w, r)
	})
}

// updateMetricsLoop периодически обновляет метрики Prometheus
func updateMetricsLoop(analyzer *analytics.Analyzer) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		avgCPU, avgRPS, _, _ := analyzer.GetStats()
		metrics.RollingAvgCPU.Set(avgCPU)
		metrics.RollingAvgRPS.Set(avgRPS)
		metrics.ActiveGoroutines.Set(float64(runtime.NumGoroutine()))
	}
}

// processAnalysisResults обрабатывает результаты анализа
func processAnalysisResults(analyzer *analytics.Analyzer, redisCache *cache.RedisCache) {
	for result := range analyzer.GetResults() {
		if result.AnomalyDetected {
			metrics.AnomaliesDetected.Inc()
			if redisCache != nil {
				redisCache.IncrementCounter("anomalies:total")
			}
			log.Printf("Anomaly detected! CPU z-score: %.2f, RPS z-score: %.2f",
				result.ZScoreCPU, result.ZScoreRPS)
		}
	}
}
