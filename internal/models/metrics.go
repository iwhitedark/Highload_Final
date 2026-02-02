// Package models содержит структуры данных для метрик и аналитики
package models

import "time"

// Metric представляет входящую метрику от IoT-устройства или API
type Metric struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`
	RPS       float64   `json:"rps"`
	DeviceID  string    `json:"device_id,omitempty"`
}

// AnalysisResult содержит результаты аналитики
type AnalysisResult struct {
	Timestamp       time.Time `json:"timestamp"`
	RollingAvgCPU   float64   `json:"rolling_avg_cpu"`
	RollingAvgRPS   float64   `json:"rolling_avg_rps"`
	ZScoreCPU       float64   `json:"z_score_cpu"`
	ZScoreRPS       float64   `json:"z_score_rps"`
	IsAnomalyCPU    bool      `json:"is_anomaly_cpu"`
	IsAnomalyRPS    bool      `json:"is_anomaly_rps"`
	AnomalyDetected bool      `json:"anomaly_detected"`
}

// MetricsBatch представляет пакет метрик для массовой загрузки
type MetricsBatch struct {
	Metrics []Metric `json:"metrics"`
}

// HealthStatus представляет статус здоровья сервиса
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Redis     string    `json:"redis"`
	Uptime    string    `json:"uptime"`
}

// StatsResponse содержит статистику сервиса
type StatsResponse struct {
	TotalMetrics     int64   `json:"total_metrics"`
	AnomaliesCount   int64   `json:"anomalies_count"`
	CurrentRPS       float64 `json:"current_rps"`
	AverageLatencyMs float64 `json:"average_latency_ms"`
}
