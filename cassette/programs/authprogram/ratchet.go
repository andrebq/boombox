package authprogram

import (
	"runtime"

	"golang.org/x/crypto/argon2"
)

type (
	Ratchet struct {
		key *Key
	}
)

func (r *Ratchet) Next(input []byte) *Key {
	var newKey Key
	// 7 passes over 10 MB should be a good replacement
	// for 1 pass over 64 MB of ram.
	buf := argon2.IDKey((*r.key)[:], input, 7, 10*1024, uint8(runtime.NumCPU()/2), uint32(len(newKey[:])))
	copy(newKey[:], buf)
	r.key = &newKey
	return r.key
}

func (r *Ratchet) Zero() {
	r.key.Zero()
}
