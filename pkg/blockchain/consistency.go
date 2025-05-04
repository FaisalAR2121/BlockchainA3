package blockchain

import (
	"sync"
	"time"
	"math"
)

// ConsistencyLevel represents the system's consistency guarantees.
type ConsistencyLevel int

const (
	Strong ConsistencyLevel = iota
	Causal
	ReadYourWrites
	Eventual
)

// NetworkMetrics captures current network conditions.
type NetworkMetrics struct {
	Latency              time.Duration
	PartitionProbability float64
	Throughput           float64
	ErrorRate            float64
}

// ConsistencyOrchestrator dynamically adjusts the consistency level
// based on observed network metrics.
type ConsistencyOrchestrator struct {
	mu               sync.RWMutex
	currentLevel     ConsistencyLevel
	networkMetrics   NetworkMetrics
	partitionHistory []float64
	latencyHistory   []time.Duration
	windowSize       int
	partitionThresh  float64
}

// NewConsistencyOrchestrator initializes with default thresholds.
func NewConsistencyOrchestrator() *ConsistencyOrchestrator {
	return &ConsistencyOrchestrator{
		currentLevel:     Strong,
		networkMetrics:   NetworkMetrics{},
		partitionHistory: make([]float64, 0),
		latencyHistory:   make([]time.Duration, 0),
		windowSize:       100,
		partitionThresh:  0.1,
	}
}

// UpdateNetworkMetrics records new metrics and adapts consistency.
func (co *ConsistencyOrchestrator) UpdateNetworkMetrics(metrics NetworkMetrics) {
	co.mu.Lock()
	defer co.mu.Unlock()

	co.networkMetrics = metrics
	co.partitionHistory = append(co.partitionHistory, metrics.PartitionProbability)
	co.latencyHistory = append(co.latencyHistory, metrics.Latency)

	if len(co.partitionHistory) > co.windowSize {
		co.partitionHistory = co.partitionHistory[1:]
	}
	if len(co.latencyHistory) > co.windowSize {
		co.latencyHistory = co.latencyHistory[1:]
	}

	co.adaptConsistencyLevel()
}

// adaptConsistencyLevel chooses a new level based on averages.
func (co *ConsistencyOrchestrator) adaptConsistencyLevel() {
	// average partition probability
	var sumP float64
	for _, p := range co.partitionHistory {
		sumP += p
	}
	avgP := sumP / float64(len(co.partitionHistory))

	// average latency
	var sumL time.Duration
	for _, l := range co.latencyHistory {
		sumL += l
	}
	avgL := time.Duration(int64(sumL) / int64(len(co.latencyHistory)))

	// choose level
	if avgP > co.partitionThresh {
		co.currentLevel = Eventual
	} else if co.networkMetrics.ErrorRate > 0.05 {
		co.currentLevel = Causal
	} else if co.networkMetrics.Throughput < 1000 {
		co.currentLevel = ReadYourWrites
	} else if avgL > 500*time.Millisecond {
		co.currentLevel = Causal
	} else {
		co.currentLevel = Strong
	}
}

// GetConsistencyLevel returns the current level.
func (co *ConsistencyOrchestrator) GetConsistencyLevel() ConsistencyLevel {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.currentLevel
}

// GetTimeout returns a backoff timeout based on error history.
func (co *ConsistencyOrchestrator) GetTimeout() time.Duration {
	co.mu.RLock()
	defer co.mu.RUnlock()

	base := 100 * time.Millisecond
	// exponential backoff factor = 2^(errorRate*10)
	factor := math.Pow(2, co.networkMetrics.ErrorRate*10)
	return time.Duration(float64(base) * factor)
}

// ShouldRetry determines retry logic under current level.
func (co *ConsistencyOrchestrator) ShouldRetry(errRate float64) bool {
	co.mu.RLock()
	defer co.mu.RUnlock()

	switch co.currentLevel {
	case Strong:
		return errRate < 0.1
	case Causal:
		return errRate < 0.2
	case ReadYourWrites:
		return errRate < 0.15
	case Eventual:
		return true
	}
	return false
}
