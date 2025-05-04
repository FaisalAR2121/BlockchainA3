package main

import (
	"fmt"
	"log"
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
	cm := blockchain.NewConsensusManager(bft)
	nodes := []string{"nodeA", "nodeB", "nodeC"}

	// Create blockchain instance
	bc := blockchain.NewBlockchain()
	defer bc.DB.Close()

	// Register shards for sync
	for shardID := range bc.Shards {
		css.RegisterShard(shardID)
	}

	// Display initialization details
	fmt.Println("=== Blockchain Initialization ===")
	fmt.Printf("Number of Shards: %d\n", len(bc.Shards))
	fmt.Printf("Current Difficulty: %s\n", cm.Difficulty().String())
	fmt.Printf("Genesis Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Println("===============================")

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
	update := css.GenerateStateUpdate(0, []byte{0}, []byte{0xFF}, leaves)
	css.BroadcastStateUpdate(update)

	// Print StateUpdate details
	fmt.Println("=== StateUpdate Details ===")
	fmt.Printf("ShardID: %d\n", update.ShardID)
	fmt.Printf("Commitment Sig: %x\n", update.Commitment.Signature)
	fmt.Printf("Proof Count: %d\n", len(update.Proof))
	for i, p := range update.Proof {
		fmt.Printf("  Proof %d: %x\n", i, p)
	}
	fmt.Println("============================")


	//provide initial trust scores
	// Bootstrap BFT scores so that nodes are trusted
	for _, n := range nodes {
		// success=true, small response time -> drives Score toward 1.0
		bft.UpdateNodeScore(n, true, 5*time.Millisecond)
	}

	// 1) Start a new dBFT round
	cm.StartRound(nodes)
	fmt.Println("Round", cm.GetRound(), "leader is", cm.GetLeader())

	// 2) Construct the block manually
	// Fetch the last block’s hash so our new block links to it
	shardID := uint32(0)
	shardBlocks := bc.GetShardBlocks(shardID)
	var prevHash []byte
	if len(shardBlocks) > 0 {
		prevHash = shardBlocks[len(shardBlocks)-1].Hash
	}
	// Now call NewBlock(transactions, prevHash, shardID)
	block := blockchain.NewBlock([]*blockchain.Transaction{tx1, tx2}, prevHash, shardID)


	// 3) Proof‐of‐Work: mine the block
	fmt.Println("Mining block with difficulty", cm.Difficulty())
	cm.MineBlock(block)
	fmt.Printf("Mined block! Nonce=%d, Hash=%x\n", block.Nonce, block.Hash)

	// 4) Leader proposes the block
	leaderID := cm.GetLeader()
	if !cm.ProposeBlock(block, leaderID) {
		log.Fatalf("Leader %q failed to propose block", leaderID)
	}

	// 5) All nodes vote
	for _, node := range nodes {
		voted := cm.Vote(block.Hash, node)
		fmt.Printf("Node %s voted: %v\n", node, voted)
	}

	// 6) Finalize the block via dBFT
	if !cm.FinalizeBlock(block) {
		log.Fatal("Block failed to reach finality")
	}
	fmt.Println("Block finalized by dBFT!")

	// 7) Commit it to the chain
	bc.AddBlock([]*blockchain.Transaction{tx1, tx2}, 0)
	fmt.Println("Block appended to blockchain.")

	// Display blocks in shard 0
	fmt.Println("=== Blocks in Shard 0 ===")
	for i, blk := range bc.GetShardBlocks(0) {
		fmt.Printf("Block %d: Hash=%x, PrevHash=%x, MerkleRoot=%x, StateRoot=%x, TxCount=%d\n",
			i, blk.Hash, blk.PrevHash, blk.MerkleRoot, blk.StateRoot, len(blk.Transactions))
	}
	fmt.Println("========================")

	// Simulate BFT leader selection (again) and print
	for _, n := range nodes {
		bft.UpdateNodeScore(n, true, 50*time.Millisecond)
	}
	fmt.Println("Selected leader:", bft.SelectLeader(nodes, 1))

	// Update metrics and print consistency
	metrics := blockchain.NetworkMetrics{
		Latency:              100 * time.Millisecond,
		PartitionProbability: 0.02,
		Throughput:           2000,
		ErrorRate:            0.01,
	}
	co.UpdateNetworkMetrics(metrics)
	fmt.Println("Consistency Level:", co.GetConsistencyLevel())
	fmt.Println("Timeout Suggestion:", co.GetTimeout())
}
