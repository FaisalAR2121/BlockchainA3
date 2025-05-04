package blockchain

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"time"
	"github.com/dgraph-io/badger/v4"
)

// Block represents a block in the blockchain
type Block struct {
	Timestamp    int64
	Transactions []*Transaction
	PrevHash     []byte
	Hash         []byte
	Nonce        int
	MerkleRoot   []byte
	StateRoot    []byte
	ShardID      uint32
	Version      uint32
	Signature    []byte
	Accumulator  []byte
}

// Transaction represents a blockchain transaction
type Transaction struct {
	ID        []byte
	Inputs    []TxInput
	Outputs   []TxOutput
	Signature []byte
	Timestamp int64
}

// TxInput represents a transaction input
type TxInput struct {
	TxID      []byte
	OutIdx    int
	Signature []byte
	PubKey    []byte
}

// TxOutput represents a transaction output
type TxOutput struct {
	Value      int64
	PubKeyHash []byte
}

// Shard represents a blockchain shard
type Shard struct {
	ID               uint32
	Blocks           []*Block
	CurrentStateRoot []byte
	TransactionCount uint64
	LastBlockHash    []byte
	StateDB          *StateDB
	Config           *ShardConfig
	mu               sync.RWMutex
}

// ShardConfig represents the configuration for sharding
type ShardConfig struct {
	MinShards         uint32
	MaxShards         uint32
	LoadThreshold     float64
	RebalanceInterval time.Duration
	MaxShardSize      uint32
	MinShardSize      uint32
}

// Blockchain represents the main blockchain structure
type Blockchain struct {
	Shards            map[uint32]*Shard
	DB                *badger.DB
	CurrentDifficulty uint64
	StateManager      *StateManager
	ShardManager      *ShardManager
	Consensus         *Consensus
	BFT               *BFT
	SyncManager       *SyncManager
}

// StateDB represents the state database for a shard
type StateDB struct {
	DB *badger.DB
}

// NewStateDB creates a new state database
func NewStateDB() *StateDB {
	return &StateDB{
		DB: nil, // In a real implementation, initialize with actual database
	}
}

// NewBlock creates and returns a new block
func NewBlock(transactions []*Transaction, prevHash []byte, shardID uint32) *Block {
	block := &Block{
		Timestamp:    time.Now().Unix(),
		Transactions: transactions,
		PrevHash:     prevHash,
		ShardID:      shardID,
		Version:      1,
	}

	// Calculate Merkle root
	block.MerkleRoot = block.CalculateMerkleRoot()

	// Calculate state root
	block.StateRoot = block.CalculateStateRoot()

	// Calculate block hash
	block.Hash = block.CalculateHash()

	return block
}

// CalculateHash calculates the hash of the block
func (b *Block) CalculateHash() []byte {
	header := bytes.Join(
		[][]byte{
			b.PrevHash,
			b.MerkleRoot,
			b.StateRoot,
			IntToHex(b.Timestamp),
			IntToHex(int64(b.Nonce)),
			IntToHex(int64(b.ShardID)),
			IntToHex(int64(b.Version)),
		},
		[]byte{},
	)

	return CalculateHash(header)
}

// CalculateMerkleRoot calculates the Merkle root of the block's transactions
func (b *Block) CalculateMerkleRoot() []byte {
    var leaves [][]byte
    for _, tx := range b.Transactions {
        leaves = append(leaves, tx.ID)
    }
    return BuildMerkleTree(leaves)
}


// CalculateStateRoot calculates the state root using cryptographic accumulator
// CalculateStateRoot builds a Merkle root over all transaction outputs
func (b *Block) CalculateStateRoot() []byte {
    var leaves [][]byte
    for _, tx := range b.Transactions {
        for _, out := range tx.Outputs {
            leaves = append(leaves, out.PubKeyHash)
        }
    }
    return BuildMerkleTree(leaves)
}


// buildMerkleTree builds a Merkle tree from a list of hashes
func buildMerkleTree(hashes [][]byte) []byte {
	if len(hashes) == 0 {
		return nil
	}

	for len(hashes) > 1 {
		var newLevel [][]byte

		for i := 0; i < len(hashes); i += 2 {
			if i+1 == len(hashes) {
				newLevel = append(newLevel, hashes[i])
				continue
			}

			combined := append(hashes[i], hashes[i+1]...)
			hash := sha256.Sum256(combined)
			newLevel = append(newLevel, hash[:])
		}

		hashes = newLevel
	}

	return hashes[0]
}

// Serialize serializes the block
func (b *Block) Serialize() []byte {
	return Serialize(b)
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) *Block {
	var block Block
	err := Deserialize(d, &block)
	if err != nil {
		return nil
	}
	return &block
}

// Consensus represents the consensus mechanism
type Consensus struct {
}

// BFT represents Byzantine Fault Tolerance
type BFT struct {
}

// SyncManager handles blockchain synchronization
type SyncManager struct {
}
