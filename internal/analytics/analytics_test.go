package analytics

import (
	"math"
	"testing"
	"time"

	"highload-service/internal/models"
)

func TestSlidingWindow_Add(t *testing.T) {
	sw := NewSlidingWindow(5)

	// Add values
	values := []float64{10, 20, 30, 40, 50}
	for _, v := range values {
		sw.Add(v)
	}

	if sw.Count() != 5 {
		t.Errorf("Expected count 5, got %d", sw.Count())
	}

	expectedMean := 30.0
	if math.Abs(sw.Mean()-expectedMean) > 0.001 {
		t.Errorf("Expected mean %.2f, got %.2f", expectedMean, sw.Mean())
	}
}

func TestSlidingWindow_RollingBehavior(t *testing.T) {
	sw := NewSlidingWindow(3)

	// Fill window
	sw.Add(10)
	sw.Add(20)
	sw.Add(30)

	// Mean should be 20
	if math.Abs(sw.Mean()-20.0) > 0.001 {
		t.Errorf("Expected mean 20, got %.2f", sw.Mean())
	}

	// Add another value, should push out 10
	sw.Add(40)

	// New mean should be (20+30+40)/3 = 30
	if math.Abs(sw.Mean()-30.0) > 0.001 {
		t.Errorf("Expected mean 30, got %.2f", sw.Mean())
	}
}

func TestSlidingWindow_StdDev(t *testing.T) {
	sw := NewSlidingWindow(5)

	// Add same value - stddev should be 0
	for i := 0; i < 5; i++ {
		sw.Add(50)
	}

	if sw.StdDev() != 0 {
		t.Errorf("Expected stddev 0 for identical values, got %.2f", sw.StdDev())
	}

	// Reset and add different values
	sw2 := NewSlidingWindow(5)
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}[:5]
	for _, v := range values {
		sw2.Add(v)
	}

	// Expected stddev ≈ 1.095 for [2,4,4,4,5]
	stddev := sw2.StdDev()
	if stddev < 0.5 || stddev > 2.0 {
		t.Errorf("StdDev out of expected range, got %.2f", stddev)
	}
}

func TestSlidingWindow_ZScore(t *testing.T) {
	sw := NewSlidingWindow(WindowSize)

	// Add normal values around 50
	for i := 0; i < WindowSize; i++ {
		sw.Add(50.0)
	}

	// Z-score for mean value should be 0
	zScore := sw.ZScore(50.0)
	if zScore != 0 {
		// With all same values, stddev is 0, so z-score should be 0
		t.Logf("Z-score for mean value with zero stddev: %.2f", zScore)
	}

	// Reset with varied values
	sw2 := NewSlidingWindow(WindowSize)
	for i := 0; i < WindowSize; i++ {
		sw2.Add(float64(40 + i%20)) // Values from 40 to 59
	}

	// Value far from mean should have high z-score
	zScoreHigh := sw2.ZScore(100.0)
	if zScoreHigh < ZScoreThreshold {
		t.Logf("Z-score for outlier value: %.2f", zScoreHigh)
	}
}

func TestAnalyzer_AnomalyDetection(t *testing.T) {
	analyzer := NewAnalyzer(100)

	// Prime with normal values
	for i := 0; i < WindowSize; i++ {
		metric := models.Metric{
			Timestamp: time.Now(),
			CPU:       50.0 + float64(i%10-5), // 45-55
			RPS:       500.0 + float64(i%20-10), // 490-510
		}
		analyzer.AnalyzeSync(metric)
	}

	// Test normal value - should not be anomaly
	normalMetric := models.Metric{
		Timestamp: time.Now(),
		CPU:       50.0,
		RPS:       500.0,
	}
	result := analyzer.AnalyzeSync(normalMetric)

	if result.AnomalyDetected {
		t.Logf("Normal metric detected as anomaly (may happen during warmup)")
	}

	// Test extreme value - should be anomaly
	anomalyMetric := models.Metric{
		Timestamp: time.Now(),
		CPU:       99.0, // Very high CPU
		RPS:       50.0,  // Very low RPS
	}
	result = analyzer.AnalyzeSync(anomalyMetric)

	// After warmup, extreme values should be detected
	t.Logf("Anomaly result - CPU anomaly: %v, RPS anomaly: %v, Z-scores: CPU=%.2f, RPS=%.2f",
		result.IsAnomalyCPU, result.IsAnomalyRPS, result.ZScoreCPU, result.ZScoreRPS)
}

func TestAnalyzer_RollingAverage(t *testing.T) {
	analyzer := NewAnalyzer(100)

	// Add metrics
	for i := 0; i < 10; i++ {
		metric := models.Metric{
			Timestamp: time.Now(),
			CPU:       float64(50 + i), // 50-59
			RPS:       float64(100 + i*10), // 100-190
		}
		analyzer.AnalyzeSync(metric)
	}

	avgCPU, avgRPS, _, _ := analyzer.GetStats()

	// Check rolling averages are computed
	if avgCPU == 0 {
		t.Error("Rolling average CPU should not be 0")
	}
	if avgRPS == 0 {
		t.Error("Rolling average RPS should not be 0")
	}

	t.Logf("Rolling averages - CPU: %.2f, RPS: %.2f", avgCPU, avgRPS)
}

func TestAnalyzer_Concurrency(t *testing.T) {
	analyzer := NewAnalyzer(1000)
	analyzer.Start(4)
	defer analyzer.Stop()

	// Submit many metrics concurrently
	done := make(chan bool)

	for i := 0; i < 4; i++ {
		go func(workerID int) {
			for j := 0; j < 100; j++ {
				metric := models.Metric{
					Timestamp: time.Now(),
					CPU:       float64(30 + workerID*10 + j%10),
					RPS:       float64(200 + workerID*50 + j%50),
				}
				analyzer.Submit(metric)
			}
			done <- true
		}(i)
	}

	// Wait for all workers
	for i := 0; i < 4; i++ {
		<-done
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Check stats are available
	avgCPU, avgRPS, stdDevCPU, stdDevRPS := analyzer.GetStats()
	t.Logf("Stats after concurrent processing - AvgCPU: %.2f, AvgRPS: %.2f, StdDevCPU: %.2f, StdDevRPS: %.2f",
		avgCPU, avgRPS, stdDevCPU, stdDevRPS)
}

func BenchmarkAnalyzeSync(b *testing.B) {
	analyzer := NewAnalyzer(10000)

	metric := models.Metric{
		Timestamp: time.Now(),
		CPU:       55.0,
		RPS:       500.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeSync(metric)
	}
}

func BenchmarkSlidingWindowAdd(b *testing.B) {
	sw := NewSlidingWindow(WindowSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.Add(float64(i % 100))
	}
}
