// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx

import (
	"encoding/binary"

	"github.com/dfinlab/meter/meter"
)

// BlockRef is block reference.
type BlockRef [8]byte

// Number extracts block number.
func (br BlockRef) Number() uint32 {
	return binary.BigEndian.Uint32(br[:])
}

func (br BlockRef) Uint64() uint64 {
	return binary.BigEndian.Uint64(br[:])
}

// NewBlockRef create block reference with block number.
func NewBlockRef(blockNum uint32) (br BlockRef) {
	binary.BigEndian.PutUint32(br[:], blockNum)
	return
}

// NewBlockRefFromID create block reference from block id.
func NewBlockRefFromID(blockID meter.Bytes32) (br BlockRef) {
	copy(br[:], blockID[:])
	return
}
