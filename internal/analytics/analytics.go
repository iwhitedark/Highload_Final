// Package analytics реализует статистический анализ метрик
// Включает rolling average для сглаживания и z-score для детекции аномалий
package analytics

import (
	"math"
	"sync"

	"highload-service/internal/models"
)

const (
	// WindowSize размер окна для rolling average и z-score (50 событий)
	WindowSize = 50
	// ZScoreThreshold порог для детекции аномалий (> 2σ)
	ZScoreThreshold = 2.0
)

// Analyzer выполняет статистический анализ метрик
type Analyzer struct {
	mu          sync.RWMutex
	cpuWindow   *SlidingWindow
	rpsWindow   *SlidingWindow
	metricsChan chan models.Metric
	resultsChan chan models.AnalysisResult
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// SlidingWindow реализует скользящее окно для хранения значений
type SlidingWindow struct {
	values []float64
	size   int
	index  int
	count  int
	sum    float64
	sumSq  float64
}

// NewSlidingWindow создает новое скользящее окно заданного размера
func NewSlidingWindow(size int) *SlidingWindow {
	return &SlidingWindow{
		values: make([]float64, size),
		size:   size,
		index:  0,
		count:  0,
		sum:    0,
		sumSq:  0,
	}
}

// Add добавляет новое значение в окно
func (sw *SlidingWindow) Add(value float64) {
	if sw.count >= sw.size {
		// Удаляем старое значение из статистики
		oldValue := sw.values[sw.index]
		sw.sum -= oldValue
		sw.sumSq -= oldValue * oldValue
	} else {
		sw.count++
	}

	// Добавляем новое значение
	sw.values[sw.index] = value
	sw.sum += value
	sw.sumSq += value * value

	sw.index = (sw.index + 1) % sw.size
}

// Mean возвращает среднее значение (rolling average)
func (sw *SlidingWindow) Mean() float64 {
	if sw.count == 0 {
		return 0
	}
	return sw.sum / float64(sw.count)
}

// StdDev возвращает стандартное отклонение
func (sw *SlidingWindow) StdDev() float64 {
	if sw.count < 2 {
		return 0
	}
	n := float64(sw.count)
	variance := (sw.sumSq - (sw.sum*sw.sum)/n) / (n - 1)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

// ZScore вычисляет z-score для заданного значения
func (sw *SlidingWindow) ZScore(value float64) float64 {
	stdDev := sw.StdDev()
	if stdDev == 0 {
		return 0
	}
	return (value - sw.Mean()) / stdDev
}

// Count возвращает количество элементов в окне
func (sw *SlidingWindow) Count() int {
	return sw.count
}

// NewAnalyzer создает новый анализатор метрик
func NewAnalyzer(bufferSize int) *Analyzer {
	return &Analyzer{
		cpuWindow:   NewSlidingWindow(WindowSize),
		rpsWindow:   NewSlidingWindow(WindowSize),
		metricsChan: make(chan models.Metric, bufferSize),
		resultsChan: make(chan models.AnalysisResult, bufferSize),
		stopChan:    make(chan struct{}),
	}
}

// Start запускает горутины для обработки метрик
func (a *Analyzer) Start(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
}

// worker горутина для обработки метрик
func (a *Analyzer) worker() {
	defer a.wg.Done()
	for {
		select {
		case metric := <-a.metricsChan:
			result := a.analyze(metric)
			select {
			case a.resultsChan <- result:
			default:
				// Канал результатов переполнен, пропускаем
			}
		case <-a.stopChan:
			return
		}
	}
}

// analyze выполняет анализ одной метрики
func (a *Analyzer) analyze(m models.Metric) models.AnalysisResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Вычисляем z-score до добавления в окно
	zScoreCPU := a.cpuWindow.ZScore(m.CPU)
	zScoreRPS := a.rpsWindow.ZScore(m.RPS)

	// Добавляем значения в окна
	a.cpuWindow.Add(m.CPU)
	a.rpsWindow.Add(m.RPS)

	// Определяем аномалии по z-score (threshold > 2σ)
	isAnomalyCPU := math.Abs(zScoreCPU) > ZScoreThreshold
	isAnomalyRPS := math.Abs(zScoreRPS) > ZScoreThreshold

	return models.AnalysisResult{
		Timestamp:       m.Timestamp,
		RollingAvgCPU:   a.cpuWindow.Mean(),
		RollingAvgRPS:   a.rpsWindow.Mean(),
		ZScoreCPU:       zScoreCPU,
		ZScoreRPS:       zScoreRPS,
		IsAnomalyCPU:    isAnomalyCPU,
		IsAnomalyRPS:    isAnomalyRPS,
		AnomalyDetected: isAnomalyCPU || isAnomalyRPS,
	}
}

// Submit отправляет метрику на обработку
func (a *Analyzer) Submit(m models.Metric) bool {
	select {
	case a.metricsChan <- m:
		return true
	default:
		return false
	}
}

// AnalyzeSync синхронно анализирует метрику
func (a *Analyzer) AnalyzeSync(m models.Metric) models.AnalysisResult {
	return a.analyze(m)
}

// GetResults возвращает канал результатов
func (a *Analyzer) GetResults() <-chan models.AnalysisResult {
	return a.resultsChan
}

// GetStats возвращает текущую статистику
func (a *Analyzer) GetStats() (avgCPU, avgRPS, stdDevCPU, stdDevRPS float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.cpuWindow.Mean(), a.rpsWindow.Mean(),
		a.cpuWindow.StdDev(), a.rpsWindow.StdDev()
}

// Stop останавливает анализатор
func (a *Analyzer) Stop() {
	close(a.stopChan)
	a.wg.Wait()
}
