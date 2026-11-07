package txn

import (
	"crypto/rand"
	"encoding/hex"
)

// NewGUID is Guid.NewGuid().ToString("N"): the 32 lowercase hexadecimal
// digits of a random version 4 UUID, which lip uses to name temporary files
// uniquely.
func NewGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails.
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return hex.EncodeToString(b[:])
}
