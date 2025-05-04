package blockchain

import "bytes"

// ZKProof represents a zero‐knowledge proof over a subtree root.
type ZKProof struct {
    Commitment []byte  // we’ll echo the SubRoot here
}

// ZKPParams is just a stub here.
type ZKPParams struct{}

// NewZKPParams returns an empty stub.
func NewZKPParams() *ZKPParams {
    return &ZKPParams{}
}

// GenerateProof “proves” the newState’s root by storing it verbatim.
func (p *ZKPParams) GenerateProof(oldState, newState *State) *ZKProof {
    return &ZKProof{
        Commitment: newState.RootHash,
    }
}

// VerifyProof succeeds if and only if the proof’s commitment matches newState.RootHash.
func (p *ZKPParams) VerifyProof(proof *ZKProof, oldState, newState *State) bool {
    if proof == nil {
        return false
    }
    return bytes.Equal(proof.Commitment, newState.RootHash)
}
