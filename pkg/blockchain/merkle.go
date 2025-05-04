package blockchain

import (
  "crypto/sha256"
)

// BuildMerkleTree builds a Merkle tree over the given data slices
// and returns the root hash.
func BuildMerkleTree(leaves [][]byte) []byte {
	if len(leaves) == 0 {
		return nil
	}
	// Copy leaves to current level
	level := make([][]byte, len(leaves))
	for i, leaf := range leaves {
		h := sha256.Sum256(leaf)
		level[i] = h[:]
	}
	// Build up until root
	for len(level) > 1 {
		if len(level)%2 != 0 {
			// duplicate last for odd count
			level = append(level, level[len(level)-1])
		}
		next := make([][]byte, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			concat := append(level[i], level[i+1]...)
			h := sha256.Sum256(concat)
			next[i/2] = h[:]
		}
		level = next
	}
	return level[0]
}

// calculateShardStateRoot computes the Merkle root over all block hashes.
func calculateShardStateRoot(blocks []*Block) []byte {
  var hashes [][]byte
  for _, blk := range blocks {
    hashes = append(hashes, blk.Hash)
  }
  return BuildMerkleTree(hashes)
}

func BuildMerkleProof(leaves [][]byte, idx int) [][]byte {
	// prepare hashed leaves
	hashes := make([][]byte, len(leaves))
	for i, leaf := range leaves {
		h := sha256.Sum256(leaf)
		hashes[i] = h[:]
	}
	proof := [][]byte{}
	// traverse up
	for count := len(hashes); count > 1; count = len(hashes) {
		if count%2 != 0 {
			hashes = append(hashes, hashes[count-1])
		}
		newHashes := [][]byte{}
		for i := 0; i < count; i += 2 {
			left, right := hashes[i], hashes[i+1]
			// record sibling
			if i == idx || i+1 == idx {
				if i == idx {
					proof = append(proof, right)
				} else {
					proof = append(proof, left)
				}
				// update idx for next level
				idx = i / 2
			}
			// hash pair
			concat := append(left, right...)
			h := sha256.Sum256(concat)
			newHashes = append(newHashes, h[:])
		}
		hashes = newHashes
	}
	return proof
}

// VerifyMerkleProof reconstructs the root from a leaf and proof, returning the computed root.
func VerifyMerkleProof(leaf []byte, proof [][]byte) []byte {
	h := sha256.Sum256(leaf)
	computed := h[:]
	for _, sibling := range proof {
		concat := append(computed, sibling...)
		h2 := sha256.Sum256(concat)
		computed = h2[:]
	}
	return computed
}
