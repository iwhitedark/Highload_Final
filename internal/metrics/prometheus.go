// Package metrics реализует экспорт метрик в Prometheus
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus метрики
var (
	// RequestsTotal общее количество запросов
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "highload_requests_total",
			Help: "Total number of requests processed",
		},
		[]string{"endpoint", "method", "status"},
	)

	// RequestDuration длительность запросов
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "highload_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"endpoint", "method"},
	)

	// MetricsReceived количество полученных метрик
	MetricsReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "highload_metrics_received_total",
			Help: "Total number of metrics received",
		},
	)

	// AnomaliesDetected количество обнаруженных аномалий
	AnomaliesDetected = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "highload_anomalies_detected_total",
			Help: "Total number of anomalies detected",
		},
	)

	// AnomalyRate скорость обнаружения аномалий
	AnomalyRate = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_anomaly_rate",
			Help: "Current anomaly rate (anomalies per minute)",
		},
	)

	// CurrentRPS текущий RPS
	CurrentRPS = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_current_rps",
			Help: "Current requests per second",
		},
	)

	// RollingAvgCPU скользящее среднее CPU
	RollingAvgCPU = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_rolling_avg_cpu",
			Help: "Rolling average of CPU usage",
		},
	)

	// RollingAvgRPS скользящее среднее RPS
	RollingAvgRPS = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_rolling_avg_rps",
			Help: "Rolling average of RPS",
		},
	)

	// ZScoreCPU z-score для CPU
	ZScoreCPU = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_zscore_cpu",
			Help: "Z-score for CPU metric",
		},
	)

	// ZScoreRPS z-score для RPS
	ZScoreRPS = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_zscore_rps",
			Help: "Z-score for RPS metric",
		},
	)

	// CacheHits попадания в кэш
	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "highload_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	// CacheMisses промахи кэша
	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "highload_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	// ActiveGoroutines количество активных горутин
	ActiveGoroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "highload_active_goroutines",
			Help: "Number of active goroutines",
		},
	)

	// AnalysisLatency время выполнения анализа
	AnalysisLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "highload_analysis_latency_seconds",
			Help:    "Analysis computation latency in seconds",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .025, .05},
		},
	)
)

// UpdateAnalysisMetrics обновляет метрики анализа
func UpdateAnalysisMetrics(avgCPU, avgRPS, zCPU, zRPS float64, isAnomaly bool) {
	RollingAvgCPU.Set(avgCPU)
	RollingAvgRPS.Set(avgRPS)
	ZScoreCPU.Set(zCPU)
	ZScoreRPS.Set(zRPS)
	if isAnomaly {
		AnomaliesDetected.Inc()
	}
}
