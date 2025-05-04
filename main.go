package main

import (
	"fmt"
	"time"
	"github.com/FaisalAR2121/BlockchainA3/pkg/blockchain"
)

func main() {
	// Initialize core components
	acc := blockchain.NewAccumulator()
	filter := blockchain.NewAMQFilter(0.01, 10)
	css := blockchain.NewCrossShardSync()
	bft := blockchain.NewBFTManager()
	co := blockchain.NewConsistencyOrchestrator()

	// Create blockchain instance
	bc := blockchain.NewBlockchain()
	defer bc.DB.Close()

	// Register each shard with the cross-shard sync manager
	for shardID := range bc.Shards {
		css.RegisterShard(shardID)
	}

	// Display initialization details
	fmt.Println("=== Blockchain Initialization ===")
	fmt.Printf("Number of Shards: %d\n", len(bc.Shards))
	fmt.Printf("Current Difficulty: %d\n", bc.CurrentDifficulty)
	fmt.Printf("Genesis Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println("===============================\n")

	// Create two example transactions
	tx1 := blockchain.NewTransaction(
		[]blockchain.TxInput{{
			TxID:      []byte("prevTx1"),
			OutIdx:    0,
			Signature: nil,
			PubKey:    []byte("pubKey1"),
		}},
		[]blockchain.TxOutput{{
			Value:      100,
			PubKeyHash: []byte("pubKeyHash1"),
		}},
	)
	tx2 := blockchain.NewTransaction(
		[]blockchain.TxInput{{
			TxID:      []byte("prevTx2"),
			OutIdx:    0,
			Signature: nil,
			PubKey:    []byte("pubKey2"),
		}},
		[]blockchain.TxOutput{{
			Value:      200,
			PubKeyHash: []byte("pubKeyHash2"),
		}},
	)

	// Display transaction info
	fmt.Println("=== Transaction Details ===")
	for i, tx := range []*blockchain.Transaction{tx1, tx2} {
		fmt.Printf("Transaction %d:\n", i+1)
		fmt.Printf("  Input:  From %x (Value: %d)\n", tx.Inputs[0].PubKey, tx.Outputs[0].Value)
		fmt.Printf("  Output: To %x (Value: %d)\n", tx.Outputs[0].PubKeyHash, tx.Outputs[0].Value)
	}
	fmt.Println("=========================")

	// Add transaction hashes to accumulator and filter
	leaves := [][]byte{tx1.Hash(), tx2.Hash()}
	for _, leaf := range leaves {
		acc.Add(leaf)
		filter.Add(leaf)
	}

	// Generate and broadcast state update for shard 0
	update := css.GenerateStateUpdate(0, nil, nil, leaves)
	css.BroadcastStateUpdate(update)

	// Print StateUpdate details
	fmt.Println("=== StateUpdate Details ===")
	fmt.Printf("ShardID: %d\n", update.ShardID)
	fmt.Printf("Commitment Sig: %x\n", update.Commitment.Signature)
	fmt.Printf("Proof Count: %d\n", len(update.Proof))
	for i, p := range update.Proof {
		fmt.Printf("  Proof %d: %x\n", i, p)
	}
	fmt.Println("============================\n")

	// Mine a new block on shard 0
	bc.AddBlock([]*blockchain.Transaction{tx1, tx2}, 0)

	// Display blocks in shard 0
	blocks := bc.GetShardBlocks(0)
	for i, blk := range blocks {
		fmt.Printf("=== Block %d in Shard %d ===\n", i, blk.ShardID)
		fmt.Printf("Hash: %x\n", blk.Hash)
		fmt.Printf("PrevHash: %x\n", blk.PrevHash)
		fmt.Printf("MerkleRoot: %x\n", blk.MerkleRoot)
		fmt.Printf("StateRoot: %x\n", blk.StateRoot)
		fmt.Printf("Timestamp: %s\n", time.Unix(blk.Timestamp, 0).Format(time.RFC3339))
		fmt.Printf("Tx Count: %d\n", len(blk.Transactions))
		fmt.Println("========================")
	}

	// Simulate BFT leader selection
	nodes := []string{"nodeA", "nodeB", "nodeC"}
	for _, n := range nodes {
		bft.UpdateNodeScore(n, true, 50*time.Millisecond)
	}
	leader := bft.SelectLeader(nodes, 1)
	fmt.Println("Selected leader:", leader)

	// Update metrics and get consistency
	metrics := blockchain.NetworkMetrics{Latency: 100 * time.Millisecond, PartitionProbability: 0.02, Throughput: 2000, ErrorRate: 0.01}
	co.UpdateNetworkMetrics(metrics)
	lvl := co.GetConsistencyLevel()
	tout := co.GetTimeout()
	fmt.Println("Consistency Level:", lvl)
	fmt.Println("Timeout Suggestion:", tout)
}
