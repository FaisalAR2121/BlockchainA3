package blockchain

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"math"
	"sync"
	"time"
)

// NodeScore represents a node's reputation score
// used for Byzantine fault tolerance.
type NodeScore struct {
	Score          float64       // current reputation score
	LastUpdate     time.Time     // last time the score was updated
	SuccessCount   int           // number of successful operations
	FailureCount   int           // number of failed operations
	MaliciousCount int           // count of detected malicious behaviors
	ResponseTime   time.Duration // last measured response time
}

// BFTManager manages node reputations and leader selection
// under Byzantine fault conditions.
type BFTManager struct {
	mu                 sync.RWMutex
	nodeScores         map[string]*NodeScore
	consensusThreshold float64
	minNodeScore       float64
	maliciousThreshold int
}

// NewBFTManager constructs a BFT manager with default parameters.
func NewBFTManager() *BFTManager {
	return &BFTManager{
		nodeScores:         make(map[string]*NodeScore),
		consensusThreshold: 0.67,
		minNodeScore:       0.5,
		maliciousThreshold: 3,
	}
}

// UpdateNodeScore updates a node's score based on success/failure and latency.
func (bft *BFTManager) UpdateNodeScore(nodeID string, success bool, responseTime time.Duration) {
	bft.mu.Lock()
	defer bft.mu.Unlock()

	score, exists := bft.nodeScores[nodeID]
	if !exists {
		score = &NodeScore{Score: 0.5, LastUpdate: time.Now()}
		bft.nodeScores[nodeID] = score
	}

	if success {
		score.SuccessCount++
	} else {
		score.FailureCount++
	}
	score.ResponseTime = responseTime

	// compute weighted score
	total := score.SuccessCount + score.FailureCount
	if total > 0 {
		sucRate := float64(score.SuccessCount) / float64(total)
		timeFactor := 1.0 - math.Min(responseTime.Seconds(), 1.0)
		score.Score = (sucRate*0.7 + timeFactor*0.3)
	}
	score.LastUpdate = time.Now()
}

// IsNodeTrusted returns true if the node's score and malicious count are acceptable.
func (bft *BFTManager) IsNodeTrusted(nodeID string) bool {
	bft.mu.RLock()
	defer bft.mu.RUnlock()

	score, exists := bft.nodeScores[nodeID]
	if !exists {
		return false
	}
	return score.Score >= bft.minNodeScore && score.MaliciousCount < bft.maliciousThreshold
}

// DetectMaliciousBehavior increases a node's malicious count and penalizes score.
func (bft *BFTManager) DetectMaliciousBehavior(nodeID, behaviorType string) {
	bft.mu.Lock()
	defer bft.mu.Unlock()

	score, exists := bft.nodeScores[nodeID]
	if !exists {
		return
	}
	score.MaliciousCount++
	switch behaviorType {
	case "double_spend":
		score.Score *= 0.5
	case "invalid_block":
		score.Score *= 0.7
	case "slow_response":
		score.Score *= 0.9
	}
}

// GetConsensusThreshold returns a dynamic threshold based on malicious nodes.
func (bft *BFTManager) GetConsensusThreshold() float64 {
	bft.mu.RLock()
	defer bft.mu.RUnlock()

	malicious := 0
	for _, sc := range bft.nodeScores {
		if sc.MaliciousCount > 0 {
			malicious++
		}
	}
	adj := float64(malicious) * 0.05
	return math.Min(bft.consensusThreshold+adj, 0.9)
}

// GenerateVRF produces a pseudo-random output and proof for a given input.
// Note: This is a stub; replace with a proper VRF implementation as needed.
func (bft *BFTManager) GenerateVRF(input []byte) (output, proof []byte) {
	h := sha256.Sum256(input)
	proof = make([]byte, 32)
	rand.Read(proof)
	return h[:], proof
}

// VerifyVRF verifies that output is correct for input (stubbed check).
func (bft *BFTManager) VerifyVRF(input, output, proof []byte) bool {
	h := sha256.Sum256(input)
	return bytes.Equal(h[:], output)
}

// SelectLeader picks a trusted node with the highest VRF output.
func (bft *BFTManager) SelectLeader(nodes []string, round int) string {
	var leader string
	var maxOut []byte
	for _, node := range nodes {
		if !bft.IsNodeTrusted(node) {
			continue
		}
		input := append([]byte(node), byte(round))
		out, _ := bft.GenerateVRF(input)
		if maxOut == nil || bytes.Compare(out, maxOut) > 0 {
			maxOut = out
			leader = node
		}
	}
	return leader
}
