package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
)

// IntToHex converts an int64 to a byte slice
func IntToHex(num int64) []byte {
	buff := make([]byte, 8)
	binary.BigEndian.PutUint64(buff, uint64(num))
	return buff
}

// CalculateHash calculates a SHA256 hash of the given data
func CalculateHash(data []byte) []byte {
	hash := sha256.Sum256(data)
	hashBytes := make([]byte, len(hash))
	copy(hashBytes, hash[:])
	return hashBytes
}

// Serialize serializes any data structure to bytes
func Serialize(data interface{}) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		return nil
	}
	return buff.Bytes()
}

// Deserialize deserializes bytes into a data structure
func Deserialize(data []byte, target interface{}) error {
	decoder := gob.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(target)
}

