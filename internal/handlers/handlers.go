// Package handlers содержит HTTP обработчики для API
package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"highload-service/internal/analytics"
	"highload-service/internal/cache"
	"highload-service/internal/metrics"
	"highload-service/internal/models"
)

// Handler содержит зависимости для HTTP обработчиков
type Handler struct {
	analyzer  *analytics.Analyzer
	cache     *cache.RedisCache
	startTime time.Time
}

// NewHandler создает новый обработчик
func NewHandler(analyzer *analytics.Analyzer, cache *cache.RedisCache) *Handler {
	return &Handler{
		analyzer:  analyzer,
		cache:     cache,
		startTime: time.Now(),
	}
}

// MetricsHandler обрабатывает POST /metrics - прием метрик
func (h *Handler) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(metrics.RequestDuration.WithLabelValues("/metrics", r.Method))
	defer timer.ObserveDuration()

	if r.Method != http.MethodPost {
		h.respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		metrics.RequestsTotal.WithLabelValues("/metrics", r.Method, "405").Inc()
		return
	}

	var metric models.Metric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		h.respondError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		metrics.RequestsTotal.WithLabelValues("/metrics", r.Method, "400").Inc()
		return
	}

	// Устанавливаем временную метку, если не указана
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	// Кэшируем метрику в Redis
	if h.cache != nil {
		if err := h.cache.CacheMetric(metric); err != nil {
			// Логируем ошибку, но продолжаем обработку
			metrics.CacheMisses.Inc()
		} else {
			metrics.CacheHits.Inc()
		}
	}

	// Отправляем на анализ
	metrics.MetricsReceived.Inc()

	// Синхронный анализ для ответа
	startAnalysis := time.Now()
	result := h.analyzer.AnalyzeSync(metric)
	metrics.AnalysisLatency.Observe(time.Since(startAnalysis).Seconds())

	// Обновляем метрики Prometheus
	metrics.UpdateAnalysisMetrics(
		result.RollingAvgCPU,
		result.RollingAvgRPS,
		result.ZScoreCPU,
		result.ZScoreRPS,
		result.AnomalyDetected,
	)

	// Кэшируем результат анализа
	if h.cache != nil {
		_ = h.cache.CacheAnalysisResult(result)
	}

	metrics.RequestsTotal.WithLabelValues("/metrics", r.Method, "200").Inc()
	h.respondJSON(w, result, http.StatusOK)
}

// AnalyzeHandler обрабатывает GET /analyze - получение статистики анализа
func (h *Handler) AnalyzeHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(metrics.RequestDuration.WithLabelValues("/analyze", r.Method))
	defer timer.ObserveDuration()

	if r.Method != http.MethodGet {
		h.respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		metrics.RequestsTotal.WithLabelValues("/analyze", r.Method, "405").Inc()
		return
	}

	avgCPU, avgRPS, stdDevCPU, stdDevRPS := h.analyzer.GetStats()

	response := map[string]interface{}{
		"timestamp":      time.Now(),
		"rolling_avg": map[string]float64{
			"cpu": avgCPU,
			"rps": avgRPS,
		},
		"std_dev": map[string]float64{
			"cpu": stdDevCPU,
			"rps": stdDevRPS,
		},
		"thresholds": map[string]float64{
			"anomaly_z_score": analytics.ZScoreThreshold,
			"window_size":     float64(analytics.WindowSize),
		},
	}

	metrics.RequestsTotal.WithLabelValues("/analyze", r.Method, "200").Inc()
	h.respondJSON(w, response, http.StatusOK)
}

// BatchMetricsHandler обрабатывает POST /metrics/batch - массовая загрузка метрик
func (h *Handler) BatchMetricsHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(metrics.RequestDuration.WithLabelValues("/metrics/batch", r.Method))
	defer timer.ObserveDuration()

	if r.Method != http.MethodPost {
		h.respondError(w, "Method not allowed", http.StatusMethodNotAllowed)
		metrics.RequestsTotal.WithLabelValues("/metrics/batch", r.Method, "405").Inc()
		return
	}

	var batch models.MetricsBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		h.respondError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		metrics.RequestsTotal.WithLabelValues("/metrics/batch", r.Method, "400").Inc()
		return
	}

	results := make([]models.AnalysisResult, 0, len(batch.Metrics))
	anomaliesCount := 0

	for _, metric := range batch.Metrics {
		if metric.Timestamp.IsZero() {
			metric.Timestamp = time.Now()
		}

		if h.cache != nil {
			_ = h.cache.CacheMetric(metric)
		}

		metrics.MetricsReceived.Inc()
		result := h.analyzer.AnalyzeSync(metric)
		results = append(results, result)

		if result.AnomalyDetected {
			anomaliesCount++
		}
	}

	response := map[string]interface{}{
		"processed":       len(batch.Metrics),
		"anomalies_found": anomaliesCount,
		"results":         results,
	}

	metrics.RequestsTotal.WithLabelValues("/metrics/batch", r.Method, "200").Inc()
	h.respondJSON(w, response, http.StatusOK)
}

// HealthHandler обрабатывает GET /health - проверка здоровья
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	redisStatus := "disconnected"
	if h.cache != nil && h.cache.Ping() == nil {
		redisStatus = "connected"
	}

	status := models.HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Redis:     redisStatus,
		Uptime:    time.Since(h.startTime).String(),
	}

	h.respondJSON(w, status, http.StatusOK)
}

// StatsHandler обрабатывает GET /stats - статистика сервиса
func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(metrics.RequestDuration.WithLabelValues("/stats", r.Method))
	defer timer.ObserveDuration()

	// Обновляем метрику горутин
	metrics.ActiveGoroutines.Set(float64(runtime.NumGoroutine()))

	var totalMetrics int64
	var anomaliesCount int64

	if h.cache != nil {
		totalMetrics, _ = h.cache.GetCounter("metrics:total")
		anomaliesCount, _ = h.cache.GetCounter("anomalies:total")
	}

	avgCPU, avgRPS, _, _ := h.analyzer.GetStats()

	response := models.StatsResponse{
		TotalMetrics:   totalMetrics,
		AnomaliesCount: anomaliesCount,
		CurrentRPS:     avgRPS,
	}

	// Обновляем Prometheus метрики
	metrics.RollingAvgCPU.Set(avgCPU)
	metrics.RollingAvgRPS.Set(avgRPS)

	metrics.RequestsTotal.WithLabelValues("/stats", r.Method, "200").Inc()
	h.respondJSON(w, response, http.StatusOK)
}

// LatestMetricsHandler возвращает последние метрики из кэша
func (h *Handler) LatestMetricsHandler(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(metrics.RequestDuration.WithLabelValues("/metrics/latest", r.Method))
	defer timer.ObserveDuration()

	count := int64(50)
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if c, err := strconv.ParseInt(countStr, 10, 64); err == nil && c > 0 && c <= 1000 {
			count = c
		}
	}

	if h.cache == nil {
		h.respondError(w, "Cache not available", http.StatusServiceUnavailable)
		return
	}

	metricsData, err := h.cache.GetLatestMetrics(count)
	if err != nil {
		h.respondError(w, "Failed to get metrics: "+err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.RequestsTotal.WithLabelValues("/metrics/latest", r.Method, "200").Inc()
	h.respondJSON(w, metricsData, http.StatusOK)
}

// respondJSON отправляет JSON ответ
func (h *Handler) respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError отправляет ошибку в JSON формате
func (h *Handler) respondError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
