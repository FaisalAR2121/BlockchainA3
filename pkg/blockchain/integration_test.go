package blockchain

import (
	"math/big"
	"testing"
	"time"
)

func TestIntegration_AllModules(t *testing.T) {
	// 1) Merkle tree & proof
	t.Log("=== Module: Merkle Tree & Proof ===")
	leaves := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	t.Logf("Leaves: %q", leaves)

	root := BuildMerkleTree(leaves)
	t.Logf("Computed Merkle Root: %x (len=%d)", root, len(root))
	if len(root) != 32 {
		t.Fatalf("expected 32-byte Merkle root, got %d bytes", len(root))
	}

	proof := BuildMerkleProof(leaves, 1)
	t.Logf("Proof for leaf #1: %x (len=%d)", proof, len(proof))
	if len(proof) == 0 {
		t.Fatal("expected non-empty Merkle proof")
	}

	recomp := VerifyMerkleProof(leaves[1], proof)
	t.Logf("Reconstructed Root: %x (len=%d)", recomp, len(recomp))
	if len(recomp) != 32 {
		t.Fatalf("expected reconstructed root of 32 bytes, got %d", len(recomp))
	}

	// 2) Accumulator & AMQFilter + GenerateProof/VerifyProof
	t.Log("=== Module: Accumulator & AMQFilter ===")
    tx := NewTransaction(
        []TxInput{{TxID: []byte("prevTx"), OutIdx: 0, Signature: nil, PubKey: []byte("pubKey")}},
        []TxOutput{{Value:  123, PubKeyHash: []byte("pubKeyHash")}},
    )
    leaf := tx.Hash()
    t.Logf("Transaction hash (leaf): %x", leaf)

    acc := NewAccumulator()
    amq := NewAMQFilter(0.1, 1)
    acc.Add(leaf)
    amq.Add(leaf)
    t.Logf("Accumulator root after Add: %x", acc.Value)
    t.Logf("AMQFilter data length: %d bits, hashCount: %d", len(amq.Data)*8, amq.hashCount)

    packed := GenerateProof(tx, acc, amq)
    t.Logf("Generated bundled proof: %x (len=%d)", packed, len(packed))
    if !VerifyProof(tx, packed, acc, amq) {
        t.Fatal("VerifyProof failed on GenerateProof output")
    }
    t.Log("VerifyProof succeeded")

	// 3) Shard lookup (single-shard scenario)
	t.Log("=== Module: ShardManager Lookup ===")
	sm := NewShardManager(ShardConfig{MaxShardSize: 10})
	sid := sm.GetShardForTransaction(tx)
	t.Logf("GetShardForTransaction returned shard ID=%d (root.ID=%d)", sid, sm.Root.ID)
	if sid != sm.Root.ID {
		t.Fatalf("expected shard %d, got %d", sm.Root.ID, sid)
	}

	// 4) Cross-shard sync: commitment & apply
	t.Log("=== Module: Cross-Shard Sync ===")
	css := NewCrossShardSync()
	css.RegisterShard(0)
	css.RegisterShard(1)

	stateRoot := []byte("dummyStateRoot")
	commit := css.CreateStateCommitment(0, stateRoot, []byte{0}, []byte{0xFF})
	t.Logf("Created StateCommitment Signature: %x", commit.Signature)
	if !css.VerifyStateCommitment(commit) {
		t.Fatal("VerifyStateCommitment failed")
	}
	t.Log("VerifyStateCommitment succeeded")

	update := &StateUpdate{
		ShardID:    0,
		RangeStart: []byte{0},
		RangeEnd:   []byte{0xFF},
		Proof:      [][]byte{},          // empty proof is OK
		SubRoots:   [][]byte{stateRoot}, // not used by ApplyStateUpdate
		Timestamp:  time.Now().Unix(),
		Commitment: commit,
	}
	applied := css.ApplyStateUpdate(update)
	t.Logf("ApplyStateUpdate returned: %v", applied)
	if !applied {
		t.Fatal("ApplyStateUpdate returned false")
	}

	// 5) BFT manager scoring & leader selection
	t.Log("=== Module: BFT Manager ===")
	bft := NewBFTManager()
	nodes := []string{"n1", "n2", "n3"}
	for _, nid := range nodes {
		bft.UpdateNodeScore(nid, true, 5*time.Millisecond)
		score := bft.nodeScores[nid].Score
		t.Logf("Node %s score after update: %.3f", nid, score)
	}
	leader := bft.SelectLeader(nodes, 1)
	t.Logf("Selected leader: %s", leader)
	if leader == "" {
		t.Fatal("SelectLeader returned empty string")
	}
	trusted := bft.IsNodeTrusted(leader)
	t.Logf("IsNodeTrusted(%s): %v", leader, trusted)
	if !trusted {
		t.Fatalf("leader %s should be trusted", leader)
	}

	// 6) Adaptive consistency orchestration
	t.Log("=== Module: Consistency Orchestrator ===")
	co := NewConsistencyOrchestrator()
	metrics := NetworkMetrics{
		Latency:              100 * time.Millisecond,
		PartitionProbability: 0.0,
		Throughput:           2000,
		ErrorRate:            0.02,
	}
	co.UpdateNetworkMetrics(metrics)
	level := co.GetConsistencyLevel()
	t.Logf("Consistency Level: %v", level)
	if level < Strong || level > Eventual {
		t.Fatalf("unexpected consistency level: %v", level)
	}
	timeout := co.GetTimeout()
	t.Logf("Suggested Timeout: %v", timeout)
	if timeout <= 0 {
		t.Fatalf("expected positive timeout, got %v", timeout)
	}
	should := co.ShouldRetry(metrics.ErrorRate)
	t.Logf("ShouldRetry(ErrorRate=%.2f): %v", metrics.ErrorRate, should)
	if !should {
		t.Fatal("ShouldRetry returned false")
	}

	// 7) ConsensusManager instantiation sanity check
	t.Log("=== Module: Consensus Manager ===")
	cm := NewConsensusManager(bft)
	if cm == nil {
		t.Fatal("NewConsensusManager returned nil")
	}
	t.Logf("Consensus difficulty: %v", cm.difficulty)
	if cm.difficulty.Cmp(big.NewInt(1)) <= 0 {
		t.Fatalf("expected difficulty > 1, got %v", cm.difficulty)
	}
}
