package blockchain

import (
	"log"

	"github.com/dgraph-io/badger/v4"
)

// NewBlockchain creates and returns a new blockchain
func NewBlockchain() *Blockchain {
	// Initialize BadgerDB
	opts := badger.DefaultOptions("")
	opts.Dir = "./data"
	opts.ValueDir = "./data"
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}

	// Create blockchain instance
	bc := &Blockchain{
		Shards:            make(map[uint32]*Shard),
		DB:                db,
		CurrentDifficulty: 1, // Initial difficulty
		StateManager:      NewStateManager(),
		ShardManager:      NewShardManager(ShardConfig{MaxShardSize: 1000}),
		Consensus:         &Consensus{},
		BFT:               &BFT{},
		SyncManager:       &SyncManager{},
	}

	// Create initial shard
	bc.createShard(0)

	// Create genesis block
	bc.AddBlock([]*Transaction{}, 0)

	return bc
}

// initializeShards initializes the shards
func (bc *Blockchain) initializeShards() {
	// Create initial shards based on a predefined number
	numShards := 4 // This can be made configurable
	for i := uint32(0); i < uint32(numShards); i++ {
		bc.createShard(i)
	}
}

// createShard creates a new shard
func (bc *Blockchain) createShard(id uint32) {
	shard := &Shard{
		ID:               id,
		Blocks:           make([]*Block, 0),
		CurrentStateRoot: []byte{},
		TransactionCount: 0,
		LastBlockHash:    []byte{},
		StateDB:          NewStateDB(),
		Config:           &ShardConfig{MaxShardSize: 1000},
	}
	bc.Shards[id] = shard
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(transactions []*Transaction, shardID uint32) {
	shard, exists := bc.Shards[shardID]
	if !exists {
		log.Panic("Shard does not exist")
	}

	var prevHash []byte
	if len(shard.Blocks) > 0 {
		prevHash = shard.Blocks[len(shard.Blocks)-1].Hash
	}

	newBlock := NewBlock(transactions, prevHash, shardID)

	err := bc.DB.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			return err
		}
		err = txn.Set([]byte("lh"), newBlock.Hash)
		if err != nil {
			return err
		}
		shard.Blocks = append(shard.Blocks, newBlock)
		return nil
	})

	if err != nil {
		log.Panic(err)
	}
}

// GetBlock finds a block by its hash
func (bc *Blockchain) GetBlock(blockHash []byte) (*Block, error) {
	var block *Block

	err := bc.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(blockHash)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			block = DeserializeBlock(val)
			return nil
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return block, nil
}

// GetShardBlocks returns all blocks in a specific shard
func (bc *Blockchain) GetShardBlocks(shardID uint32) []*Block {
	shard, exists := bc.Shards[shardID]
	if !exists {
		return nil
	}

	return shard.Blocks
}

// NewGenesisBlock creates and returns genesis Block
func NewGenesisBlock() *Block {
	return NewBlock([]*Transaction{}, []byte{}, 0)
}

// GetCurrentStateRoot returns the current state root of the blockchain
func (bc *Blockchain) GetCurrentStateRoot() []byte {
	// For simplicity, return the state root of the first shard
	// In a real implementation, you might want to combine state roots from all shards
	if len(bc.Shards) > 0 {
		if shard, exists := bc.Shards[0]; exists {
			return shard.CurrentStateRoot
		}
	}
	return nil
}
