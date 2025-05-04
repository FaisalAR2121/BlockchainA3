package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// CrossShardSync manages cross-shard state synchronization.
type CrossShardSync struct {
	mu            sync.RWMutex
	stateChannels map[uint32]chan *StateUpdate
	commitments   map[string]*StateCommitment
	zkpParams     *ZKPParams
	vectorClocks  map[uint32]*VectorClock
}

// StateUpdate represents a partial state update for a specific key‐range.
type StateUpdate struct {
	ShardID    uint32
	RangeStart []byte
	RangeEnd   []byte
	Leaves     [][]byte     // state entries (leaf hashes) in [RangeStart,RangeEnd]
	Proof      [][]byte     // serialized Merkle proof blobs, one per leaf
	ZKProof    *ZKProof     // structured zero-knowledge proof
	SubRoot    []byte       // Merkle root over the Leaves
	VClock     *VectorClock // causal timestamp snapshot
	Timestamp  int64
	Commitment *StateCommitment // cryptographic commitment to this subrange
}

// StateCommitment is a cryptographic commitment to a state range.
type StateCommitment struct {
	RootHash   []byte
	RangeStart []byte
	RangeEnd   []byte
	Timestamp  int64
	Signature  []byte
}

// NewCrossShardSync initializes a new cross‐shard sync manager.
func NewCrossShardSync() *CrossShardSync {
	return &CrossShardSync{
		stateChannels: make(map[uint32]chan *StateUpdate),
		commitments:   make(map[string]*StateCommitment),
		zkpParams:     NewZKPParams(),
		vectorClocks:  make(map[uint32]*VectorClock),
	}
}

// RegisterShard registers a shard ID to receive updates, initializing its clock.
func (css *CrossShardSync) RegisterShard(shardID uint32) {
	css.mu.Lock()
	defer css.mu.Unlock()
	css.stateChannels[shardID] = make(chan *StateUpdate, 100)
	css.vectorClocks[shardID] = NewVectorClock()
}

// BroadcastStateUpdate sends an update to all other registered shards.
func (css *CrossShardSync) BroadcastStateUpdate(update *StateUpdate) {
	css.mu.RLock()
	defer css.mu.RUnlock()
	for id, ch := range css.stateChannels {
		if id != update.ShardID {
			select {
			case ch <- update:
			default:
				// skip if channel buffer is full
			}
		}
	}
}

// CreateStateCommitment commits to a subrange [start,end] and its Merkle root.
func (css *CrossShardSync) CreateStateCommitment(shardID uint32, root, start, end []byte) *StateCommitment {
	c := &StateCommitment{
		RootHash:   root,
		RangeStart: start,
		RangeEnd:   end,
		Timestamp:  time.Now().Unix(),
	}
	h := sha256.New()
	binary.Write(h, binary.BigEndian, c.Timestamp)
	h.Write(c.RangeStart)
	h.Write(c.RangeEnd)
	h.Write(c.RootHash)
	c.Signature = h.Sum(nil)

	css.mu.Lock()
	css.commitments[string(c.Signature)] = c
	css.mu.Unlock()

	return c
}

// VerifyStateCommitment checks integrity and existence of a commitment.
func (css *CrossShardSync) VerifyStateCommitment(c *StateCommitment) bool {
	h := sha256.New()
	binary.Write(h, binary.BigEndian, c.Timestamp)
	h.Write(c.RangeStart)
	h.Write(c.RangeEnd)
	h.Write(c.RootHash)
	expected := h.Sum(nil)
	if !bytes.Equal(expected, c.Signature) {
		return false
	}
	css.mu.RLock()
	defer css.mu.RUnlock()
	_, ok := css.commitments[string(c.Signature)]
	return ok
}

// GenerateStateUpdate constructs a partial‐state update with Merkle proofs, ZK proof, and VClock.
func (css *CrossShardSync) GenerateStateUpdate(
	shardID uint32,
	start, end []byte,
	leaves [][]byte,
) *StateUpdate {
	// 1) Merkle sub-root
	subRoot := BuildMerkleTree(leaves)

	// 2) Per-leaf Merkle proofs
	proofs := make([][]byte, len(leaves))
	for i := range leaves {
		proofHashes := BuildMerkleProof(leaves, i)
		proofs[i] = concatProof(proofHashes)
	}

	// 3) Commitment for this range
	commit := css.CreateStateCommitment(shardID, subRoot, start, end)

	// 4) ZK proof over the new state root
	oldState := &State{RootHash: nil}
	newState := &State{RootHash: subRoot}
	zkproof := css.zkpParams.GenerateProof(oldState, newState)

	// 5) Advance & snapshot the shard’s vector clock
	vc := css.vectorClocks[shardID]
	vc.Increment(fmt.Sprint(shardID))
	snapshot := NewVectorClock()
	snapshot.Update(vc)

	return &StateUpdate{
		ShardID:    shardID,
		RangeStart: start,
		RangeEnd:   end,
		Leaves:     leaves,
		Proof:      proofs,
		ZKProof:    zkproof,
		SubRoot:    subRoot,
		VClock:     snapshot,
		Timestamp:  time.Now().Unix(),
		Commitment: commit,
	}
}

// ApplyStateUpdate verifies Merkle proofs, ZK proof, and causal ordering.
func (css *CrossShardSync) ApplyStateUpdate(update *StateUpdate) bool {
	// A) Commitment integrity
	if !css.VerifyStateCommitment(update.Commitment) {
		return false
	}
	// B) Merkle proof checks
	for i, blob := range update.Proof {
		proofHashes := decodeProof(blob)
		leaf := update.Leaves[i]
		root := VerifyMerkleProof(leaf, proofHashes)
		if !bytes.Equal(root, update.Commitment.RootHash) {
			return false
		}
	}
	// C) Zero-knowledge proof
	oldState := &State{RootHash: nil}
	newState := &State{RootHash: update.SubRoot}
	if !css.zkpParams.VerifyProof(update.ZKProof, oldState, newState) {
		return false
	}
	// D) Causal consistency: reject stale updates
	local := css.vectorClocks[update.ShardID]
	if local.Compare(update.VClock) == 1 {
		// local clock > update clock => stale
		return false
	}
	// Merge clocks for future updates
	local.Update(update.VClock)

	// TODO: apply update.Leaves to the local StateDB
	return true
}

// func (css *CrossShardSync) ApplyStateUpdate(update *StateUpdate) bool {
//     return true
// }

// concatProof flattens multiple 32-byte hashes into one byte slice.
func concatProof(proof [][]byte) []byte {
	var out []byte
	for _, p := range proof {
		out = append(out, p...)
	}
	return out
}

// decodeProof splits a flat byte slice back into 32-byte hashes.
func decodeProof(data []byte) [][]byte {
	var proofs [][]byte
	for i := 0; i+32 <= len(data); i += 32 {
		proofs = append(proofs, data[i:i+32])
	}
	return proofs
}
