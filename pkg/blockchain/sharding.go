package blockchain

import (
	"bytes"
	"math/big"
	"sync"
	"sync/atomic"
)

var shardIDCounter uint32 = 1

// ShardNode represents one node in the Adaptive Merkle Forest.
type ShardNode struct {
	Shard                // underlying shard data (blocks, state DB, etc.)
	RangeStart, RangeEnd []byte

	Left, Right *ShardNode
	mu          sync.RWMutex
}

// ShardManager now holds a forest root rather than a flat map.
type ShardManager struct {
	Root       *ShardNode
	Config     ShardConfig
}

// NewShardManager creates a manager with one root shard covering the full keyspace.
func NewShardManager(cfg ShardConfig) *ShardManager {
	sm := &ShardManager{Config: cfg}
	fullStart := make([]byte, 32)
	fullEnd := bytes.Repeat([]byte{0xFF}, 32)
	// initialize root node covering 0x00..00 - 0xFF..FF
	sm.Root = &ShardNode{
		Shard:      *newShard(0, &sm.Config),
		RangeStart: fullStart,
		RangeEnd:   fullEnd,
	}
	return sm
}

// findNode descends the tree to locate the leaf shard for a given hash.
func (sm *ShardManager) findNode(node *ShardNode, hash []byte) *ShardNode {
	node.mu.RLock()
	defer node.mu.RUnlock()
	// if leaf, return
	if node.Left == nil && node.Right == nil {
		return node
	}
	// decide branch by midpoint of this node's range
	mid := midpoint(node.RangeStart, node.RangeEnd)
	if bytes.Compare(hash, mid) <= 0 {
		return sm.findNode(node.Left, hash)
	}
	return sm.findNode(node.Right, hash)
}

// GetShardForTransaction uses the AMF lookup to pick the correct shard ID.
func (sm *ShardManager) GetShardForTransaction(tx *Transaction) uint32 {
	h := tx.Hash()
	leaf := sm.findNode(sm.Root, h)
	return leaf.ID
}

// SplitShard splits a leaf node into two at its hash-range midpoint.
func (sm *ShardManager) SplitShard(node *ShardNode) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.Left != nil || node.Right != nil {
		return // already split
	}
	mid := midpoint(node.RangeStart, node.RangeEnd)

	// create left child
	left := &ShardNode{
		Shard:      *newShard(nextID(), &sm.Config),
		RangeStart: node.RangeStart,
		RangeEnd:   mid,
	}
	// create right child
	right := &ShardNode{
		Shard:      *newShard(nextID(), &sm.Config),
		RangeStart: mid,
		RangeEnd:   node.RangeEnd,
	}
	// distribute existing blocks by their block hash
	for _, blk := range node.Blocks {
		target := left
		if bytes.Compare(blk.Hash, mid) > 0 {
			target = right
		}
		target.Blocks = append(target.Blocks, blk)
	}
	// compute state roots
	left.CurrentStateRoot = calculateShardStateRoot(left.Blocks)
	right.CurrentStateRoot = calculateShardStateRoot(right.Blocks)

	// attach children and clear parent blocks
	node.Left, node.Right = left, right
	node.Blocks = nil
}

// MergeShard merges a pair of leaf siblings back into their parent.
func (sm *ShardManager) MergeShard(parent *ShardNode) {
	parent.mu.Lock()
	defer parent.mu.Unlock()
	if parent.Left == nil || parent.Right == nil {
		return
	}
	// recombine blocks
	parent.Blocks = append(parent.Left.Blocks, parent.Right.Blocks...)
	parent.CurrentStateRoot = calculateShardStateRoot(parent.Blocks)
	parent.Left, parent.Right = nil, nil
}

// newShard initializes a Shard bound to the given config.
func newShard(id uint32, cfg *ShardConfig) *Shard {
	return &Shard{
		ID:               id,
		Blocks:           make([]*Block, 0),
		CurrentStateRoot: []byte{},
		TransactionCount: 0,
		LastBlockHash:    []byte{},
		StateDB:          NewStateDB(),
		Config:           cfg,
	}
}

// midpoint computes the numeric midpoint of two equal-length big-endian slices.
func midpoint(a, b []byte) []byte {
	ai := new(big.Int).SetBytes(a)
	bi := new(big.Int).SetBytes(b)
	sum := new(big.Int).Add(ai, bi)
	mid := sum.Rsh(sum, 1)
	return mid.FillBytes(make([]byte, len(a)))
}

// nextID atomically generates a new shard ID.
func nextID() uint32 {
	return atomic.AddUint32(&shardIDCounter, 1)
}
