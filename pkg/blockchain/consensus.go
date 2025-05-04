package blockchain

import (
  "bytes"
//   "crypto/sha256"
  "math/big"
  "math/rand"
  "sync"
  "time"
)

// maxTarget is the maximum possible 256-bit value (2^256−1).
var maxTarget *big.Int

func init() {
	maxTarget = new(big.Int).Sub(
		new(big.Int).Lsh(big.NewInt(1), 256),
		big.NewInt(1),
	)
}

// ConsensusManager manages the hybrid PoW + dBFT consensus.
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

// NewConsensusManager creates a new consensus manager.
// Starts with difficulty 2^16 and target = maxTarget / difficulty.
func NewConsensusManager(bftManager *BFTManager) *ConsensusManager {
	difficulty := new(big.Int).Lsh(big.NewInt(1), 16) // 2^16
	target := new(big.Int).Div(maxTarget, difficulty)

	return &ConsensusManager{
		bftManager:    bftManager,
		difficulty:    difficulty,
		target:        target,
		roundDuration: 10 * time.Second,
		currentRound:  0,
		votes:         make(map[string]int),
		proposals:     make(map[string]*Block),
		lastBlockTime: time.Now(),
	}
}

// MineBlock runs proof-of-work: finds a nonce s.t. H(block||nonce) < target.
// It sets block.Nonce and block.Hash when found.
func (cm *ConsensusManager) MineBlock(b *Block) {
	for {
		b.Nonce = rand.Uint64()
		h := b.CalculateHash()
		if new(big.Int).SetBytes(h).Cmp(cm.target) == -1 {
			b.Hash = h
			return
		}
	}
}

// StartRound begins a new dBFT voting round among the given nodes.
func (cm *ConsensusManager) StartRound(nodes []string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.currentRound++
	cm.votes = make(map[string]int)
	cm.proposals = make(map[string]*Block)
	cm.leader = cm.bftManager.SelectLeader(nodes, cm.currentRound)
	cm.lastBlockTime = time.Now()
}

// ProposeBlock is called by the leader to register its mined block.
func (cm *ConsensusManager) ProposeBlock(block *Block, nodeID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if nodeID != cm.leader {
		return false
	}
	// Verify that the block’s PoW meets the target
	if !cm.verifyPoW(block) {
		return false
	}
	cm.proposals[nodeID] = block
	return true
}

// Vote allows any trusted node to vote on a proposed block.
func (cm *ConsensusManager) Vote(blockHash []byte, nodeID string) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.bftManager.IsNodeTrusted(nodeID) {
		return false
	}
	cm.votes[string(blockHash)]++
	return cm.checkConsensus(blockHash)
}

// verifyPoW checks that block.Hash (or CalculateHash) < target.
func (cm *ConsensusManager) verifyPoW(b *Block) bool {
	hash := b.CalculateHash()
	hashInt := new(big.Int).SetBytes(hash)
	return hashInt.Cmp(cm.target) == -1
}

// checkConsensus returns true once votes for blockHash reach the BFT threshold.
func (cm *ConsensusManager) checkConsensus(blockHash []byte) bool {
	totalVotes := 0
	for _, v := range cm.votes {
		totalVotes += v
	}
	if totalVotes == 0 {
		return false
	}
	needed := cm.bftManager.GetConsensusThreshold()
	ratio := float64(cm.votes[string(blockHash)]) / float64(totalVotes)
	return ratio >= needed
}

// FinalizeBlock returns true if the block has been proposed and voted through.
func (cm *ConsensusManager) FinalizeBlock(block *Block) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Ensure leader proposed it
	proposed, ok := cm.proposals[cm.leader]
	if !ok || !bytes.Equal(proposed.Hash, block.Hash) {
		return false
	}
	return cm.checkConsensus(block.Hash)
}

// AdjustDifficulty tunes difficulty based on time since last block.
func (cm *ConsensusManager) AdjustDifficulty() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	elapsed := time.Since(cm.lastBlockTime)
	targetTime := 10 * time.Second

	// coarsely adjust difficulty by ±1
	switch {
	case elapsed > targetTime*2:
		cm.difficulty.Sub(cm.difficulty, big.NewInt(1))
	case elapsed < targetTime/2:
		cm.difficulty.Add(cm.difficulty, big.NewInt(1))
	}
	if cm.difficulty.Cmp(big.NewInt(1)) < 0 {
		cm.difficulty.SetInt64(1)
	}
	// recompute target = maxTarget / difficulty
	cm.target = new(big.Int).Div(maxTarget, cm.difficulty)
}

// GetLeader returns the current round’s leader.
func (cm *ConsensusManager) GetLeader() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.leader
}

// GetRound returns the current round number.
func (cm *ConsensusManager) GetRound() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.currentRound
}

// Difficulty returns the current difficulty.
func (cm *ConsensusManager) Difficulty() *big.Int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return new(big.Int).Set(cm.difficulty)
}
