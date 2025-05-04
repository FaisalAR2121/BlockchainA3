package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"time"
)

// CrossShardSync manages cross-shard state synchronization
type CrossShardSync struct {
	mu            sync.RWMutex
	stateChannels map[uint32]chan *StateUpdate
	commitments   map[string]*StateCommitment
}

// StateUpdate represents a partial state update for a specific key-range
type StateUpdate struct {
	ShardID    uint32
	RangeStart []byte
	RangeEnd   []byte
	Proof      [][]byte         // serialized Merkle proofs for each included leaf
	SubRoots   [][]byte         // subtree roots corresponding to each leaf set
	Timestamp  int64
	Commitment *StateCommitment // cryptographic commitment
}

// StateCommitment represents a cryptographic commitment to a state range
type StateCommitment struct {
	RootHash   []byte
	RangeStart []byte
	RangeEnd   []byte
	Timestamp  int64
	Signature  []byte
}

// NewCrossShardSync creates a new cross-shard synchronization manager
func NewCrossShardSync() *CrossShardSync {
	return &CrossShardSync{
		stateChannels: make(map[uint32]chan *StateUpdate),
		commitments:   make(map[string]*StateCommitment),
	}
}

// RegisterShard registers a shard for synchronization
func (css *CrossShardSync) RegisterShard(shardID uint32) {
	css.mu.Lock()
	defer css.mu.Unlock()
	css.stateChannels[shardID] = make(chan *StateUpdate, 100)
}

// BroadcastStateUpdate sends a state update to all other shards
func (css *CrossShardSync) BroadcastStateUpdate(update *StateUpdate) {
	css.mu.RLock()
	defer css.mu.RUnlock()
	for id, ch := range css.stateChannels {
		if id != update.ShardID {
			select {
			case ch <- update:
			default:
				// skip if full
			}
		}
	}
}

// CreateStateCommitment creates a cryptographic commitment to a state range
func (css *CrossShardSync) CreateStateCommitment(shardID uint32, root, start, end []byte) *StateCommitment {
	c := &StateCommitment{
		RootHash:   root,
		RangeStart: start,
		RangeEnd:   end,
		Timestamp:  time.Now().Unix(),
	}
	// compute signature = H(Timestamp || RangeStart || RangeEnd || RootHash)
	h := sha256.New()
	binary.Write(h, binary.BigEndian, c.Timestamp)
	h.Write(c.RangeStart)
	h.Write(c.RangeEnd)
	h.Write(c.RootHash)
	c.Signature = h.Sum(nil)
	// store commitment
	css.mu.Lock()
	css.commitments[string(c.Signature)] = c
	css.mu.Unlock()
	return c
}

// VerifyStateCommitment verifies a state commitment's integrity and existence
func (css *CrossShardSync) VerifyStateCommitment(c *StateCommitment) bool {
	// recompute signature
	h := sha256.New()
	binary.Write(h, binary.BigEndian, c.Timestamp)
	h.Write(c.RangeStart)
	h.Write(c.RangeEnd)
	h.Write(c.RootHash)
	expected := h.Sum(nil)
	if !bytes.Equal(expected, c.Signature) {
		return false
	}
	// ensure stored
	css.mu.RLock()
	defer css.mu.RUnlock()
	_, ok := css.commitments[string(c.Signature)]
	return ok
}

// GenerateStateUpdate produces a StateUpdate for a given shard and key-range
type GenerateStateUpdate func(shardID uint32, start, end []byte, leaves [][]byte) *StateUpdate

func (css *CrossShardSync) GenerateStateUpdate(shardID uint32, start, end []byte, leaves [][]byte) *StateUpdate {
	proofs := make([][]byte, len(leaves))
	for i := range leaves {
		p := BuildMerkleProof(leaves, i)
		proofs[i] = concatProof(p)
	}
	root := BuildMerkleTree(leaves)
	commit := css.CreateStateCommitment(shardID, root, start, end)
	return &StateUpdate{
		ShardID:    shardID,
		RangeStart: start,
		RangeEnd:   end,
		Proof:      proofs,
		SubRoots:   [][]byte{root},
		Timestamp:  time.Now().Unix(),
		Commitment: commit,
	}
}

// ApplyStateUpdate verifies and applies a received StateUpdate
func (css *CrossShardSync) ApplyStateUpdate(update *StateUpdate) bool {
	if !css.VerifyStateCommitment(update.Commitment) {
		return false
	}
	// verify each proof matches commitment root
	for _, p := range update.Proof {
		proofHashes := decodeProof(p)
		root := VerifyMerkleProof(nil, proofHashes)
		if !bytes.Equal(root, update.Commitment.RootHash) {
			return false
		}
	}
	// integrate update into local state store here
	return true
}

// concatProof serializes a slice of byte slices into one flat []byte
func concatProof(proof [][]byte) []byte {
	var out []byte
	for _, p := range proof {
		out = append(out, p...)
	}
	return out
}

// decodeProof splits a flat proof blob into fixed-size 32-byte hashes
func decodeProof(data []byte) [][]byte {
	var proofs [][]byte
	for i := 0; i+32 <= len(data); i += 32 {
		proofs = append(proofs, data[i:i+32])
	}
	return proofs
}
