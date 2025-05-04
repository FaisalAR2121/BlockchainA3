package blockchain

import (
	"bytes"
	"math"
	"sync"
	"time"
)

// VectorClock represents a vector clock for causal ordering
type VectorClock struct {
	mu     sync.RWMutex
	clocks map[string]uint64
}

// NewVectorClock creates a new vector clock
func NewVectorClock() *VectorClock {
	return &VectorClock{
		clocks: make(map[string]uint64),
	}
}

// Increment increments the clock for a node
func (vc *VectorClock) Increment(nodeID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[nodeID]++
}

// Update updates the vector clock with another clock
func (vc *VectorClock) Update(other *VectorClock) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	for nodeID, value := range other.clocks {
		if vc.clocks[nodeID] < value {
			vc.clocks[nodeID] = value
		}
	}
}

// Compare compares two vector clocks
func (vc *VectorClock) Compare(other *VectorClock) int {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	// Check if one clock is strictly less than the other
	less := true
	greater := true

	for nodeID, value := range vc.clocks {
		otherValue := other.clocks[nodeID]
		if value > otherValue {
			less = false
		}
		if value < otherValue {
			greater = false
		}
	}

	if less && !greater {
		return -1
	}
	if greater && !less {
		return 1
	}
	return 0
}

// Conflict represents a detected conflict
type Conflict struct {
	Transaction1 *Transaction
	Transaction2 *Transaction
	Type         string
	Timestamp    time.Time
	Entropy      float64
}

// ConflictResolver manages conflict resolution
type ConflictResolver struct {
	mu            sync.RWMutex
	conflicts     map[string]*Conflict
	vectorClocks  map[string]*VectorClock
	entropyWindow int
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver() *ConflictResolver {
	return &ConflictResolver{
		conflicts:     make(map[string]*Conflict),
		vectorClocks:  make(map[string]*VectorClock),
		entropyWindow: 100,
	}
}

// DetectConflict detects conflicts between transactions
func (cr *ConflictResolver) DetectConflict(tx1, tx2 *Transaction) *Conflict {
	// Check for double spending
	if bytes.Equal(tx1.Inputs[0].TxID, tx2.Inputs[0].TxID) {
		return &Conflict{
			Transaction1: tx1,
			Transaction2: tx2,
			Type:         "double_spend",
			Timestamp:    time.Now(),
			Entropy:      cr.calculateEntropy(tx1, tx2),
		}
	}

	// Check for conflicting state updates
	if cr.checkStateConflict(tx1, tx2) {
		return &Conflict{
			Transaction1: tx1,
			Transaction2: tx2,
			Type:         "state_conflict",
			Timestamp:    time.Now(),
			Entropy:      cr.calculateEntropy(tx1, tx2),
		}
	}

	return nil
}

// checkStateConflict checks for state conflicts
func (cr *ConflictResolver) checkStateConflict(tx1, tx2 *Transaction) bool {
	// Check if transactions modify the same state
	tx1State := make(map[string]bool)
	tx2State := make(map[string]bool)

	// Add modified accounts to state sets
	for _, input := range tx1.Inputs {
		tx1State[string(input.PubKey)] = true
	}
	for _, output := range tx1.Outputs {
		tx1State[string(output.PubKeyHash)] = true
	}

	for _, input := range tx2.Inputs {
		tx2State[string(input.PubKey)] = true
	}
	for _, output := range tx2.Outputs {
		tx2State[string(output.PubKeyHash)] = true
	}

	// Check for intersection
	for addr := range tx1State {
		if tx2State[addr] {
			return true
		}
	}

	return false
}

// calculateEntropy calculates the entropy of a conflict
func (cr *ConflictResolver) calculateEntropy(tx1, tx2 *Transaction) float64 {
	// Calculate entropy based on transaction properties
	valueDiff := math.Abs(float64(tx1.Outputs[0].Value - tx2.Outputs[0].Value))
	timeDiff := math.Abs(float64(tx1.Timestamp - tx2.Timestamp))

	// Normalize values
	maxValue := math.Max(float64(tx1.Outputs[0].Value), float64(tx2.Outputs[0].Value))
	valueEntropy := valueDiff / maxValue
	timeEntropy := timeDiff / float64(time.Hour)

	// Combine entropies
	return (valueEntropy + timeEntropy) / 2
}

// ResolveConflict resolves a detected conflict
func (cr *ConflictResolver) ResolveConflict(conflict *Conflict) *Transaction {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Store conflict
	conflictKey := string(conflict.Transaction1.Hash()) + string(conflict.Transaction2.Hash())
	cr.conflicts[conflictKey] = conflict

	// Get vector clocks
	vc1 := cr.getVectorClock(conflict.Transaction1)
	vc2 := cr.getVectorClock(conflict.Transaction2)

	// Compare vector clocks
	comparison := vc1.Compare(vc2)

	// Resolve based on conflict type and entropy
	switch conflict.Type {
	case "double_spend":
		if comparison == 1 {
			return conflict.Transaction1
		} else if comparison == -1 {
			return conflict.Transaction2
		} else {
			// If clocks are concurrent, use entropy
			if conflict.Entropy > 0.5 {
				return conflict.Transaction1
			}
			return conflict.Transaction2
		}
	case "state_conflict":
		// For state conflicts, prefer the transaction with higher entropy
		if conflict.Entropy > 0.5 {
			return conflict.Transaction1
		}
		return conflict.Transaction2
	}

	return nil
}

// getVectorClock gets or creates a vector clock for a transaction
func (cr *ConflictResolver) getVectorClock(tx *Transaction) *VectorClock {
	txHash := string(tx.Hash())
	vc, exists := cr.vectorClocks[txHash]
	if !exists {
		vc = NewVectorClock()
		cr.vectorClocks[txHash] = vc
	}
	return vc
}

// UpdateVectorClock updates the vector clock for a transaction
func (cr *ConflictResolver) UpdateVectorClock(tx *Transaction, nodeID string) {
	vc := cr.getVectorClock(tx)
	vc.Increment(nodeID)
}
