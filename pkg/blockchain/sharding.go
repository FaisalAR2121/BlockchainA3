package blockchain

import (
  "bytes"
//   "encoding/binary"
  "math/big"
  "sync"
  "sync/atomic"
)

// ShardNode is a node in our Adaptive Merkle Forest.
type ShardNode struct {
  Shard
  RangeStart, RangeEnd []byte
  Left, Right          *ShardNode
  mu                   sync.RWMutex
}

type ShardManager struct {
  Root        *ShardNode
  Config      ShardConfig
  nextShardID uint32
}

// package‐level counter for shard IDs (alternative to manager field)
// var shardIDCounter uint32 = 0

// NewShardManager bootstraps one root shard covering the entire keyspace.
func NewShardManager(cfg ShardConfig) *ShardManager {
  fullStart := make([]byte, 32)
  fullEnd := bytes.Repeat([]byte{0xFF}, 32)

  sm := &ShardManager{
    Config:      cfg,
    nextShardID: 1, // start IDs at 1
  }
  root := newShard(sm.nextID(), &sm.Config, fullStart, fullEnd)
  sm.Root = root
  return sm
}

// nextID atomically hands out a new shard ID.
func (sm *ShardManager) nextID() uint32 {
  return atomic.AddUint32(&sm.nextShardID, 1)
}

// GetShardForTransaction hashes the tx, then finds the leaf node in O(log N).
func (sm *ShardManager) GetShardForTransaction(tx *Transaction) uint32 {
  h := tx.Hash()
  leaf := sm.findNode(sm.Root, h)
  return leaf.ID
}

// findNode traverses the AMF tree to locate the proper leaf shard.
func (sm *ShardManager) findNode(node *ShardNode, hash []byte) *ShardNode {
  node.mu.RLock()
  defer node.mu.RUnlock()
  if node.Left == nil && node.Right == nil {
    return node
  }
  mid := midpoint(node.RangeStart, node.RangeEnd)
  if bytes.Compare(hash, mid) <= 0 {
    return sm.findNode(node.Left, hash)
  }
  return sm.findNode(node.Right, hash)
}

// SplitShard splits a leaf into two child shards at the range midpoint.
func (sm *ShardManager) SplitShard(node *ShardNode) {
  node.mu.Lock()
  defer node.mu.Unlock()
  if node.Left != nil || node.Right != nil {
    return // already split
  }
  mid := midpoint(node.RangeStart, node.RangeEnd)
  left := newShard(sm.nextID(), &sm.Config, node.RangeStart, mid)
  right := newShard(sm.nextID(), &sm.Config, mid, node.RangeEnd)

  // Re‐distribute existing blocks by hash
  for _, blk := range node.Blocks {
    target := left
    if bytes.Compare(blk.Hash, mid) > 0 {
      target = right
    }
    target.Blocks = append(target.Blocks, blk)
  }
  // Recompute state‐roots
  left.CurrentStateRoot = calculateShardStateRoot(left.Blocks)
  right.CurrentStateRoot = calculateShardStateRoot(right.Blocks)

  node.Left, node.Right = left, right
  node.Blocks = nil // optional: free parent’s blocks
}

// MergeShard collapses two leaf siblings back into their parent.
func (sm *ShardManager) MergeShard(parent *ShardNode) {
  parent.mu.Lock()
  defer parent.mu.Unlock()
  if parent.Left == nil || parent.Right == nil {
    return
  }
  parent.Blocks = append(parent.Left.Blocks, parent.Right.Blocks...)
  parent.CurrentStateRoot = calculateShardStateRoot(parent.Blocks)
  parent.Left, parent.Right = nil, nil
}

// newShard creates a ShardNode (leaf) for the given key‐range.
func newShard(id uint32, cfg *ShardConfig, start, end []byte) *ShardNode {
  return &ShardNode{
    Shard: Shard{
      ID:               id,
      Blocks:           make([]*Block, 0),
      CurrentStateRoot: []byte{},
      TransactionCount: 0,
      LastBlockHash:    []byte{},
      StateDB:          NewStateDB(),
      Config:           cfg,
    },
    RangeStart: start,
    RangeEnd:   end,
  }
}

// midpoint computes the numeric midpoint of two equal‐length big‐endian byte slices.
func midpoint(a, b []byte) []byte {
  ai := new(big.Int).SetBytes(a)
  bi := new(big.Int).SetBytes(b)
  sum := new(big.Int).Add(ai, bi)
  mid := sum.Rsh(sum, 1)
  return mid.FillBytes(make([]byte, len(a)))
}
