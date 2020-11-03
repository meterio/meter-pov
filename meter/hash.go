// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"fmt"
	"hash"

	"golang.org/x/crypto/blake2b"
)

// NewBlake2b return blake2b-256 hash.
func NewBlake2b() hash.Hash {
	hash, err := blake2b.New256(nil)
	if err != nil {
		fmt.Println("could not new blake2b 256, error:", err)
	}
	return hash
}

// Blake2b computes blake2b-256 checksum for given data.
func Blake2b(data ...[]byte) (b32 Bytes32) {
	hash := NewBlake2b()
	for _, b := range data {
		_, err := hash.Write(b)
		if err != nil {
			fmt.Println("could not write hash, error:", err)
		}

	}
	hash.Sum(b32[:0])
	return
}
