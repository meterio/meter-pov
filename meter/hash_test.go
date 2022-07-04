// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter

import (
	"testing"

	"golang.org/x/crypto/sha3"
	//"golang.org/x/crypto/blake2b"

	"github.com/meterio/meter-pov/crypto/blake2b"
)

func BenchmarkKeccak(b *testing.B) {
	data := []byte("hello world")
	for i := 0; i < b.N; i++ {
		hash := sha3.NewLegacyKeccak256()
		hash.Write(data)
		hash.Sum(nil)
	}
}

func BenchmarkBlake2b(b *testing.B) {
	data := []byte("hello world")
	for i := 0; i < b.N; i++ {
		hash, _ := blake2b.New256(nil)
		hash.Write(data)
		hash.Sum(nil)
	}
}
