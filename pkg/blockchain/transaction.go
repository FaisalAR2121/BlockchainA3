package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"time"
)

// NewTransaction creates a new transaction
func NewTransaction(inputs []TxInput, outputs []TxOutput) *Transaction {
	tx := &Transaction{
		ID:        nil,
		Inputs:    inputs,
		Outputs:   outputs,
		Signature: nil,
		Timestamp: time.Now().Unix(),
	}
	tx.ID = tx.Hash()
	return tx
}

// Hash returns the hash of the Transaction
func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.ID = []byte{}
	txCopy.Signature = nil

	hash = sha256.Sum256(txCopy.Serialize())

	return hash[:]
}

// Sign signs a transaction
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey) {
	if tx.ID == nil {
		tx.ID = tx.Hash()
	}

	r, s, err := ecdsa.Sign(rand.Reader, &privKey, tx.ID)
	if err != nil {
		log.Panic(err)
	}

	signature := append(r.Bytes(), s.Bytes()...)
	tx.Signature = signature
}

// Verify verifies a transaction
func (tx *Transaction) Verify(pubKey ecdsa.PublicKey) bool {
	curve := elliptic.P256()
	r := new(ecdsa.PublicKey)
	r.Curve = curve
	r.X = pubKey.X
	r.Y = pubKey.Y

	rInt := new(ecdsa.PublicKey)
	sInt := new(ecdsa.PublicKey)
	rInt.X = r.X
	rInt.Y = r.Y
	sInt.X = r.X
	sInt.Y = r.Y

	return ecdsa.Verify(&pubKey, tx.ID, rInt.X, sInt.X)
}

// Serialize serializes a transaction
func (tx *Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

// DeserializeTransaction deserializes a transaction
func DeserializeTransaction(data []byte) *Transaction {
	var transaction Transaction

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	if err != nil {
		log.Panic(err)
	}

	return &transaction
}
