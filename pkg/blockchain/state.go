package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"time"
)

// StateManager manages blockchain state
type StateManager struct {
	mu               sync.RWMutex
	currentState     *State
	stateHistory     []*State
	pruningInterval  time.Duration
	lastPruneTime    time.Time
	compressionRatio float64
}

// State represents the blockchain state
type State struct {
	RootHash    []byte
	Accounts    map[string]*Account
	Contracts   map[string]*Contract
	Timestamp   int64
	Version     uint32
	Accumulator *Accumulator
}

// Account represents a blockchain account
type Account struct {
	Balance     uint64
	Nonce       uint64
	CodeHash    []byte
	StorageRoot []byte
}

// Contract represents a smart contract
type Contract struct {
	Code        []byte
	Storage     map[string][]byte
	LastUpdated int64
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		currentState: &State{
			Accounts:    make(map[string]*Account),
			Contracts:   make(map[string]*Contract),
			Accumulator: NewAccumulator(),
		},
		stateHistory:     make([]*State, 0),
		pruningInterval:  24 * time.Hour,
		compressionRatio: 0.5,
	}
}

// UpdateState updates the current state
func (sm *StateManager) UpdateState(transactions []*Transaction) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Create new state
	newState := &State{
		Accounts:    make(map[string]*Account),
		Contracts:   make(map[string]*Contract),
		Timestamp:   time.Now().Unix(),
		Version:     sm.currentState.Version + 1,
		Accumulator: NewAccumulator(),
	}

	// Copy current state
	for addr, acc := range sm.currentState.Accounts {
		newState.Accounts[addr] = &Account{
			Balance:     acc.Balance,
			Nonce:       acc.Nonce,
			CodeHash:    acc.CodeHash,
			StorageRoot: acc.StorageRoot,
		}
	}

	for addr, contract := range sm.currentState.Contracts {
		newState.Contracts[addr] = &Contract{
			Code:        contract.Code,
			Storage:     make(map[string][]byte),
			LastUpdated: contract.LastUpdated,
		}
		for key, value := range contract.Storage {
			newState.Contracts[addr].Storage[key] = value
		}
	}

	// Apply transactions
	for _, tx := range transactions {
		sm.applyTransaction(newState, tx)
	}

	// Calculate new state root
	newState.RootHash = sm.calculateStateRoot(newState)

	// Update accumulator
	for _, tx := range transactions {
		newState.Accumulator.Add(tx.Hash())
	}

	// Update current state
	sm.currentState = newState
	sm.stateHistory = append(sm.stateHistory, newState)

	// Check if pruning is needed
	if time.Since(sm.lastPruneTime) > sm.pruningInterval {
		sm.pruneState()
	}
}

// applyTransaction applies a transaction to the state
func (sm *StateManager) applyTransaction(state *State, tx *Transaction) {
	// Update sender account
	sender := state.Accounts[string(tx.Inputs[0].PubKey)]
	sender.Balance -= uint64(tx.Outputs[0].Value)
	sender.Nonce++

	// Update receiver account
	receiver := state.Accounts[string(tx.Outputs[0].PubKeyHash)]
	if receiver == nil {
		receiver = &Account{}
		state.Accounts[string(tx.Outputs[0].PubKeyHash)] = receiver
	}
	receiver.Balance += uint64(tx.Outputs[0].Value)
}

// calculateStateRoot calculates the state root hash
func (sm *StateManager) calculateStateRoot(state *State) []byte {
	// Create a Merkle tree of account states
	var hashes [][]byte
	for addr, acc := range state.Accounts {
		// Hash account data
		hash := sha256.New()
		binary.Write(hash, binary.BigEndian, acc.Balance)
		binary.Write(hash, binary.BigEndian, acc.Nonce)
		hash.Write(acc.CodeHash)
		hash.Write(acc.StorageRoot)
		hash.Write([]byte(addr))
		hashes = append(hashes, hash.Sum(nil))
	}

	// Build Merkle tree
	return buildMerkleTree(hashes)
}

// pruneState prunes old state history
func (sm *StateManager) pruneState() {
	// Calculate number of states to keep
	keepCount := int(float64(len(sm.stateHistory)) * sm.compressionRatio)
	if keepCount < 1 {
		keepCount = 1
	}

	// Keep only the most recent states
	sm.stateHistory = sm.stateHistory[len(sm.stateHistory)-keepCount:]
	sm.lastPruneTime = time.Now()
}

// GetState returns the current state
func (sm *StateManager) GetState() *State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// GetStateAt returns the state at a specific version
func (sm *StateManager) GetStateAt(version uint32) *State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for i := len(sm.stateHistory) - 1; i >= 0; i-- {
		if sm.stateHistory[i].Version == version {
			return sm.stateHistory[i]
		}
	}

	return nil
}

// VerifyState verifies the integrity of a state
func (sm *StateManager) VerifyState(state *State) bool {
    // 1. Recompute the state root and compare
    calculatedRoot := sm.calculateStateRoot(state)
    if !bytes.Equal(calculatedRoot, state.RootHash) {
        return false
    }

    // 2. For each account entry, prove & verify its inclusion in the accumulator
    for _, tx := range state.Accounts {
        proof, found := state.Accumulator.ProveMembership(tx.CodeHash)
        if !found {
            return false
        }
        if !state.Accumulator.VerifyProof(tx.CodeHash, proof) {
            return false
        }
    }

    return true
}
