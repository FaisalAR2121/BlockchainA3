package blockchain

import (
	"math/big"
	"sync"
	"time"
)

// ConsensusManager manages the hybrid consensus protocol
type ConsensusManager struct {
	mu            sync.RWMutex
	bftManager    *BFTManager
	difficulty    *big.Int
	target        *big.Int
	roundDuration time.Duration
	currentRound  int
	leader        string
	votes         map[string]int
	proposals     map[string]*Block
	lastBlockTime time.Time
}

// NewConsensusManager creates a new consensus manager
func NewConsensusManager(bftManager *BFTManager) *ConsensusManager {
	target := big.NewInt(1)
	target.Lsh(target, 256-16) // Adjust difficulty as needed

	return &ConsensusManager{
		bftManager:    bftManager,
		difficulty:    big.NewInt(16), // Adjust as needed
		target:        target,
		roundDuration: 10 * time.Second,
		currentRound:  0,
		votes:         make(map[string]int),
		proposals:     make(map[string]*Block),
		lastBlockTime: time.Now(),
	}
}

// StartRound starts a new consensus round
func (cm *ConsensusManager) StartRound(nodes []string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.currentRound++
	cm.votes = make(map[string]int)
	cm.proposals = make(map[string]*Block)

	// Select leader using VRF
	cm.leader = cm.bftManager.SelectLeader(nodes, cm.currentRound)
}

// ProposeBlock proposes a new block
func (cm *ConsensusManager) ProposeBlock(block *Block, nodeID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Only leader can propose in the first phase
	if nodeID != cm.leader {
		return false
	}

	// Verify PoW
	if !cm.verifyPoW(block) {
		return false
	}

	cm.proposals[nodeID] = block
	return true
}

// Vote votes on a proposed block
func (cm *ConsensusManager) Vote(blockHash []byte, nodeID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if node is trusted
	if !cm.bftManager.IsNodeTrusted(nodeID) {
		return false
	}

	// Increment vote count
	cm.votes[string(blockHash)]++

	// Check if consensus reached
	return cm.checkConsensus(blockHash)
}

// verifyPoW verifies the proof of work
func (cm *ConsensusManager) verifyPoW(block *Block) bool {
	// Calculate block hash
	hash := block.CalculateHash()

	// Convert hash to big integer
	hashInt := new(big.Int).SetBytes(hash)

	// Check if hash meets target difficulty
	return hashInt.Cmp(cm.target) == -1
}

// checkConsensus checks if consensus is reached
func (cm *ConsensusManager) checkConsensus(blockHash []byte) bool {
	totalVotes := 0
	for _, count := range cm.votes {
		totalVotes += count
	}

	// Get current consensus threshold
	threshold := cm.bftManager.GetConsensusThreshold()

	// Check if votes exceed threshold
	voteRatio := float64(cm.votes[string(blockHash)]) / float64(totalVotes)
	return voteRatio >= threshold
}

// GetLeader returns the current round leader
func (cm *ConsensusManager) GetLeader() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.leader
}

// GetRound returns the current round number
func (cm *ConsensusManager) GetRound() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.currentRound
}

// AdjustDifficulty adjusts the PoW difficulty
func (cm *ConsensusManager) AdjustDifficulty() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Calculate time since last block
	timeSinceLast := time.Since(cm.lastBlockTime)
	targetTime := 10 * time.Second // Adjust as needed

	// Adjust difficulty based on block time
	if timeSinceLast > targetTime*2 {
		cm.difficulty.Sub(cm.difficulty, big.NewInt(1))
	} else if timeSinceLast < targetTime/2 {
		cm.difficulty.Add(cm.difficulty, big.NewInt(1))
	}

	// Update target
	cm.target = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	cm.target.Div(cm.target, cm.difficulty)
}
