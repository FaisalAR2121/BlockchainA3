package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// Accumulator represents a cryptographic accumulator using a Merkle tree.
type Accumulator struct {
	leaves [][]byte
	Value  []byte // current Merkle root
}

// NewAccumulator creates a new accumulator (empty Merkle tree).
func NewAccumulator() *Accumulator {
	return &Accumulator{
		leaves: make([][]byte, 0),
		Value:  nil,
	}
}

// Add inserts an element into the accumulator and updates the Merkle root.
func (a *Accumulator) Add(element []byte) {
	// hash the element as a leaf
	h := sha256.Sum256(element)
	a.leaves = append(a.leaves, h[:])
	// rebuild the Merkle root
	a.Value = BuildMerkleTree(a.leaves)
}

// ProveMembership returns a Merkle proof for an element, and a flag if found.
func (a *Accumulator) ProveMembership(element []byte) (proof [][]byte, found bool) {
	// hash the element for lookup
	h := sha256.Sum256(element)
	for idx, leaf := range a.leaves {
		if bytes.Equal(leaf, h[:]) {
			proof = BuildMerkleProof(a.leaves, idx)
			return proof, true
		}
	}
	return nil, false
}

// VerifyProof checks that the proof for element leads to the stored root.
func (a *Accumulator) VerifyProof(element []byte, proof [][]byte) bool {
	h := sha256.Sum256(element)
	reconstructed := VerifyMerkleProof(h[:], proof)
	return bytes.Equal(reconstructed, a.Value)
}

// AMQFilter represents a Bloom-filter--style AMQ filter.
type AMQFilter struct {
	Data              []byte
	FalsePositiveRate float64
	hashCount         int
}

// NewAMQFilter creates a new AMQ filter with optimal size and hash count.
func NewAMQFilter(falsePositiveRate float64, expectedElements int) *AMQFilter {
	// m = -(n * ln(p)) / (ln2)^2, k = (m/n)*ln2
	m := int(math.Ceil(-1 * float64(expectedElements) * math.Log(falsePositiveRate) / (math.Ln2 * math.Ln2)))
	k := int(math.Ceil((float64(m) / float64(expectedElements)) * math.Ln2))
	return &AMQFilter{
		Data:              make([]byte, m),
		FalsePositiveRate: falsePositiveRate,
		hashCount:         k,
	}
}

// Add inserts an element into the AMQ filter.
func (f *AMQFilter) Add(element []byte) {
	for i := 0; i < f.hashCount; i++ {
		hashInput := append(element, byte(i))
		h := sha256.Sum256(hashInput)
		idx := binary.BigEndian.Uint32(h[:]) % uint32(len(f.Data))
		f.Data[idx] = 1
	}
}

// Contains checks if an element might be in the filter (or definitely not).
func (f *AMQFilter) Contains(element []byte) bool {
	for i := 0; i < f.hashCount; i++ {
		hashInput := append(element, byte(i))
		h := sha256.Sum256(hashInput)
		idx := binary.BigEndian.Uint32(h[:]) % uint32(len(f.Data))
		if f.Data[idx] == 0 {
			return false
		}
	}
	return true
}

// GenerateProof bundles accumulator proof, filter snapshot, and tx hash.
func GenerateProof(tx *Transaction, acc *Accumulator, filter *AMQFilter) []byte {
	var buf []byte
	// include Merkle proof for this tx
	proof, found := acc.ProveMembership(tx.Hash())
	if !found {
		proof = [][]byte{}
	}
	// serialize proof count + each hash
	count := uint32(len(proof))
	cntBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(cntBytes, count)
	buf = append(buf, cntBytes...)
	for _, p := range proof {
		buf = append(buf, p...)
	}
	// include filter data
	buf = append(buf, filter.Data...)
	// include tx hash
	txHash := tx.Hash()
	buf = append(buf, txHash...)
	return buf
}

// VerifyProof verifies the bundled proof against accumulator and filter.
func VerifyProof(tx *Transaction, proof []byte, acc *Accumulator, filter *AMQFilter) bool {
	// read proof count
	if len(proof) < 4 {
		return false
	}
	count := binary.BigEndian.Uint32(proof[:4])
	off := 4
	// extract Merkle proof
	var merkleProof [][]byte
	for i := uint32(0); i < count; i++ {
		if len(proof) < off+32 {
			return false
		}
		merkleProof = append(merkleProof, proof[off:off+32])
		off += 32
	}
	// verify accumulator proof
	if !acc.VerifyProof(tx.Hash(), merkleProof) {
		return false
	}
	// check filter
	fDataLen := len(filter.Data)
	if len(proof) < off+fDataLen+32 {
		return false
	}
	filterBytes := proof[off : off+fDataLen]
	if !bytes.Equal(filterBytes, filter.Data) {
		return false
	}
	off += fDataLen
	// verify tx hash
	txHash := proof[off : off+32]
	if !bytes.Equal(tx.Hash(), txHash) {
		return false
	}
	return true
}