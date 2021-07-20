// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tx

import (
	"encoding/binary"
	"fmt"
	"hash"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/crypto/sha3"
)

type bytesBacked interface {
	Bytes() []byte
}

const (
	// EthBloomByteLength represents the number of bytes used in a header log bloom.
	EthBloomByteLength = 256

	// EthBloomBitLength represents the number of bits used in a header log bloom.
	EthBloomBitLength = 8 * EthBloomByteLength
)

// EthBloom represents a 2048 bit bloom filter.
type EthBloom [EthBloomByteLength]byte

// BytesToEthBloom converts a byte slice to a bloom filter.
// It panics if b is not of suitable size.
func BytesToEthBloom(b []byte) EthBloom {
	var bloom EthBloom
	bloom.SetBytes(b)
	return bloom
}

// SetBytes sets the content of b to the given bytes.
// It panics if d is not of suitable size.
func (b *EthBloom) SetBytes(d []byte) {
	if len(b) < len(d) {
		panic(fmt.Sprintf("bloom bytes too big %d %d", len(b), len(d)))
	}
	copy(b[EthBloomByteLength-len(d):], d)
}

// Add adds d to the filter. Future calls of Test(d) will return true.
func (b *EthBloom) Add(d []byte) {
	b.add(d, make([]byte, 6))
}

// add is internal version of Add, which takes a scratch buffer for reuse (needs to be at least 6 bytes)
func (b *EthBloom) add(d []byte, buf []byte) {
	i1, v1, i2, v2, i3, v3 := bloomValues(d, buf)
	b[i1] |= v1
	b[i2] |= v2
	b[i3] |= v3
}

// Big converts b to a big integer.
// Note: Converting a bloom filter to a big.Int and then calling GetBytes
// does not return the same bytes, since big.Int will trim leading zeroes
func (b EthBloom) Big() *big.Int {
	return new(big.Int).SetBytes(b[:])
}

// Bytes returns the backing byte slice of the bloom
func (b EthBloom) Bytes() []byte {
	return b[:]
}

// Test checks if the given topic is present in the bloom filter
func (b EthBloom) Test(topic []byte) bool {
	i1, v1, i2, v2, i3, v3 := bloomValues(topic, make([]byte, 6))
	return v1 == v1&b[i1] &&
		v2 == v2&b[i2] &&
		v3 == v3&b[i3]
}

// MarshalText encodes b as a hex string with 0x prefix.
func (b EthBloom) MarshalText() ([]byte, error) {
	return hexutil.Bytes(b[:]).MarshalText()
}

// UnmarshalText b as a hex string with 0x prefix.
func (b *EthBloom) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("EthBloom", input, b[:])
}

// CreateEthBloom creates a bloom filter out of the give Receipts (+Logs)
func CreateEthBloom(receipts Receipts) EthBloom {
	buf := make([]byte, 6)
	var bin EthBloom
	for _, receipt := range receipts {
		for _, out := range receipt.Outputs {
			for _, log := range out.Events {
				bin.add(log.Address.Bytes(), buf)
				for _, b := range log.Topics {
					bin.add(b[:], buf)
				}
			}
		}
	}
	return bin
}

// LogsEthBloom returns the bloom bytes for the given logs
// func LogsEthBloom(logs []*Log) []byte {
// 	buf := make([]byte, 6)
// 	var bin EthBloom
// 	for _, log := range logs {
// 		bin.add(log.Address.Bytes(), buf)
// 		for _, b := range log.Topics {
// 			bin.add(b[:], buf)
// 		}
// 	}
// 	return bin[:]
// }

// EthBloom9 returns the bloom filter for the given data
func EthBloom9(data []byte) []byte {
	var b EthBloom
	b.SetBytes(data)
	return b.Bytes()
}

// KeccakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

// NewKeccakState creates a new KeccakState
func NewKeccakState() KeccakState {
	return sha3.NewLegacyKeccak256().(KeccakState)
}

// HashData hashes the provided data using the KeccakState and returns a 32 byte hash
func HashData(kh KeccakState, data []byte) (h common.Hash) {
	kh.Reset()
	kh.Write(data)
	kh.Read(h[:])
	return h
}

// hasherPool holds LegacyKeccak hashers.
var hasherPool = sync.Pool{
	New: func() interface{} {
		return sha3.NewLegacyKeccak256()
	},
}

// bloomValues returns the bytes (index-value pairs) to set for the given data
func bloomValues(data []byte, hashbuf []byte) (uint, byte, uint, byte, uint, byte) {
	sha := hasherPool.Get().(KeccakState)
	sha.Reset()
	sha.Write(data)
	sha.Read(hashbuf)
	hasherPool.Put(sha)
	// The actual bits to flip
	v1 := byte(1 << (hashbuf[1] & 0x7))
	v2 := byte(1 << (hashbuf[3] & 0x7))
	v3 := byte(1 << (hashbuf[5] & 0x7))
	// The indices for the bytes to OR in
	i1 := EthBloomByteLength - uint((binary.BigEndian.Uint16(hashbuf)&0x7ff)>>3) - 1
	i2 := EthBloomByteLength - uint((binary.BigEndian.Uint16(hashbuf[2:])&0x7ff)>>3) - 1
	i3 := EthBloomByteLength - uint((binary.BigEndian.Uint16(hashbuf[4:])&0x7ff)>>3) - 1

	return i1, v1, i2, v2, i3, v3
}

// EthBloomLookup is a convenience-method to check presence int he bloom filter
func EthBloomLookup(bin EthBloom, topic bytesBacked) bool {
	return bin.Test(topic.Bytes())
}
