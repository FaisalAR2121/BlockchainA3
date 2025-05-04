package blockchain

import (
	"math/big"
	"testing"
	"time"
)

func TestIntegration_AllModules(t *testing.T) {
	// 1) Merkle tree & proof
	leaves := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	root := BuildMerkleTree(leaves)
	if len(root) != 32 {
		t.Fatalf("expected 32-byte Merkle root, got %d bytes", len(root))
	}
	proof := BuildMerkleProof(leaves, 1)
	if len(proof) == 0 {
		t.Fatal("expected non-empty Merkle proof")
	}
	recomp := VerifyMerkleProof(leaves[1], proof)
	if len(recomp) != 32 {
		t.Fatalf("expected proof to reconstruct a 32-byte hash, got %d bytes", len(recomp))
	}

	// 2) GenerateProof/VerifyProof: Accumulator & AMQFilter must contain the tx hash
	// Create a sample transaction
	tx := NewTransaction(
		[]TxInput{{TxID: []byte("prev"), OutIdx: 0, Signature: nil, PubKey: []byte("pk")}},
		[]TxOutput{{Value:  42, PubKeyHash: []byte("pkh")}},
	)
	// Build accumulator/filter over exactly that tx hash
	acc := NewAccumulator()
	amq := NewAMQFilter(0.1, 1)
	leaf := tx.Hash()
	acc.Add(leaf)
	amq.Add(leaf)

	// Now generate and verify the bundled proof
	packed := GenerateProof(tx, acc, amq)
	if !VerifyProof(tx, packed, acc, amq) {
		t.Fatal("VerifyProof failed on GenerateProof output")
	}

	// 3) Shard lookup (single-shard)
	sm := NewShardManager(ShardConfig{MaxShardSize: 5})
	shardID := sm.GetShardForTransaction(tx)
	if shardID != sm.Root.ID {
		t.Fatalf("expected shard %d, got %d", sm.Root.ID, shardID)
	}

	// 4) Cross-shard sync: commitment + apply
	css := NewCrossShardSync()
	// Register two shards so we can apply an update
	css.RegisterShard(0)
	css.RegisterShard(1)

	stateRoot := []byte("dummyRoot")
	commit := css.CreateStateCommitment(0, stateRoot, []byte{0}, []byte{0xFF})
	if !css.VerifyStateCommitment(commit) {
		t.Fatal("VerifyStateCommitment failed")
	}
	update := &StateUpdate{
		ShardID:    0,
		RangeStart: []byte{0},
		RangeEnd:   []byte{0xFF},
		Proof:      [][]byte{},          // empty proof → skipped
		SubRoots:   [][]byte{stateRoot}, // not actually used in ApplyStateUpdate
		Timestamp:  time.Now().Unix(),
		Commitment: commit,
	}
	if !css.ApplyStateUpdate(update) {
		t.Fatal("ApplyStateUpdate returned false")
	}

	// 5) BFT: scoring & leader selection
	bft := NewBFTManager()
	nodes := []string{"n1", "n2", "n3"}
	for _, n := range nodes {
		bft.UpdateNodeScore(n, true, 10*time.Millisecond)
	}
	leader := bft.SelectLeader(nodes, 1)
	if leader == "" {
		t.Fatal("SelectLeader returned empty string")
	}
	if !bft.IsNodeTrusted(leader) {
		t.Fatalf("leader %s should be trusted", leader)
	}

	// 6) Adaptive consistency
	co := NewConsistencyOrchestrator()
	metrics := NetworkMetrics{
		Latency:              50 * time.Millisecond,
		PartitionProbability: 0.0,
		Throughput:           1000,
		ErrorRate:            0.01,
	}
	co.UpdateNetworkMetrics(metrics)
	level := co.GetConsistencyLevel()
	if level < Strong || level > Eventual {
		t.Fatalf("unexpected consistency level: %v", level)
	}
	timeout := co.GetTimeout()
	if timeout <= 0 {
		t.Fatalf("expected positive timeout, got %v", timeout)
	}
	if !co.ShouldRetry(metrics.ErrorRate) {
		t.Fatal("ShouldRetry returned false")
	}

	// 7) ConsensusManager instantiation sanity check
	cm := NewConsensusManager(bft)
	if cm == nil {
		t.Fatal("NewConsensusManager returned nil")
	}
	if cm.difficulty.Cmp(big.NewInt(1)) <= 0 {
		t.Fatalf("expected difficulty > 1, got %v", cm.difficulty)
	}
}
