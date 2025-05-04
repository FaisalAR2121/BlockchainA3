package blockchain

import (
	"bytes"
	"testing"
	"time"
)

func TestIntegration_AllModules(t *testing.T) {
	// 1. Merkle tree & proof
	leaves := [][]byte{[]byte("leaf1"), []byte("leaf2"), []byte("leaf3")}
	root := BuildMerkleTree(leaves)
	if len(root) != 32 {
		t.Fatalf("expected 32-byte Merkle root, got %d bytes", len(root))
	}
	proof := BuildMerkleProof(leaves, 1) // proof for "leaf2"
	if len(proof) == 0 {
		t.Fatal("expected non-empty proof for leaf2")
	}
	if !bytes.Equal(VerifyMerkleProof(leaves[1], proof), root) {
		t.Fatal("Merkle proof did not reconstruct the root")
	}

	// 2. Accumulator & AMQFilter
	acc := NewAccumulator()
	fil := NewAMQFilter(0.05, 3)
	for _, leaf := range leaves {
		acc.Add(leaf)
		fil.Add(leaf)
	}
	// Prove & verify membership in accumulator
	merkleProof, found := acc.ProveMembership(leaves[2])
	if !found {
		t.Fatal("expected leaf3 to be found in accumulator")
	}
	if !acc.VerifyProof(leaves[2], merkleProof) {
		t.Fatal("accumulator proof failed to verify")
	}
	// Test global VerifyProof with a dummy transaction
	tx := NewTransaction(
		[]TxInput{{TxID: []byte("id"), OutIdx: 0, Signature: nil, PubKey: []byte("pk")}},
		[]TxOutput{{Value: 1, PubKeyHash: leaves[2]}},
	)
	// build a bundled proof: [count][merkleProof...][filter.Data][tx.Hash()]
	bundled := func() []byte {
		p := make([]byte, 4)
		binary.BigEndian.PutUint32(p, uint32(len(merkleProof)))
		for _, leafHash := range merkleProof {
			p = append(p, leafHash...)
		}
		p = append(p, fil.Data...)
		p = append(p, tx.Hash()...)
		return p
	}()
	if !VerifyProof(tx, bundled, acc, fil) {
		t.Fatal("VerifyProof(tx…) failed")
	}

	// 3. ShardManager lookup (single-shard case)
	sm := NewShardManager(ShardConfig{MaxShardSize: 10})
	tx2 := &Transaction{ID: leaves[0]}
	sid := sm.GetShardForTransaction(tx2)
	if sid != sm.Root.ID {
		t.Fatalf("expected shard %d for tx, got %d", sm.Root.ID, sid)
	}

	// 4. Cross-shard sync: commitment + broadcast + process
	css := NewCrossShardSync()
	// register two shards so BroadcastStateUpdate actually sends
	css.RegisterShard(0)
	css.RegisterShard(1)

	stateRoot := []byte("state-root")
	commit := css.CreateStateCommitment(0, stateRoot)
	if !css.VerifyStateCommitment(commit) {
		t.Fatal("VerifyStateCommitment failed")
	}

	update := &StateUpdate{
		ShardID:    0,
		StateRoot:  stateRoot,
		Proof:      []byte("dummyProof"),
		Timestamp:  time.Now().Unix(),
		Commitment: commit,
	}
	css.BroadcastStateUpdate(update)

	// Expect the update on shard 1’s channel
	ch1 := css.GetStateChannel(1)
	select {
	case u := <-ch1:
		if !css.ProcessStateUpdate(u) {
			t.Fatal("ProcessStateUpdate returned false")
		}
	default:
		t.Fatal("expected a state update on shard 1’s channel")
	}

	// 5. BFT leader selection
	bft := NewBFTManager()
	nodes := []string{"nA", "nB", "nC"}
	for _, n := range nodes {
		bft.UpdateNodeScore(n, true, 5*time.Millisecond)
	}
	leader := bft.SelectLeader(nodes, 42)
	if leader == "" {
		t.Fatal("SelectLeader returned empty leader")
	}

	// 6. Consistency adaptation
	co := NewConsistencyOrchestrator()
	metrics := NetworkMetrics{
		Latency:              100 * time.Millisecond,
		PartitionProbability: 0.02,
		Throughput:           2000,
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
	if !co.ShouldRetry(nil) {
		t.Fatalf("ShouldRetry(nil) returned false at level %v", level)
	}
}
