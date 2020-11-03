// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
)

// XXX: Yang: Builder only build header and txs. evidence/committee info and kblock data built by app.
// Builder to make it easy to build a block object.
type Builder struct {
	headerBody HeaderBody
	txs        tx.Transactions
	//	committeeInfo CommitteeInfo
	//	kBlockData    kBlockData
}

// ParentID set parent id.
func (b *Builder) ParentID(id meter.Bytes32) *Builder {
	b.headerBody.ParentID = id
	return b
}

// LastKBlockID set last KBlock id.
func (b *Builder) LastKBlockHeight(height uint32) *Builder {
	b.headerBody.LastKBlockHeight = height
	return b
}

// Timestamp set timestamp.
func (b *Builder) Timestamp(ts uint64) *Builder {
	b.headerBody.Timestamp = ts
	return b
}

// BlockType set block type BLOCK_TYPE_K_BLOCK/BLOCK_TYPE_M_BLOCK.
func (b *Builder) BlockType(t uint32) *Builder {
	b.headerBody.BlockType = t
	return b
}

// TotalScore set total score.
func (b *Builder) TotalScore(score uint64) *Builder {
	b.headerBody.TotalScore = score
	return b
}

// GasLimit set gas limit.
func (b *Builder) GasLimit(limit uint64) *Builder {
	b.headerBody.GasLimit = limit
	return b
}

// GasUsed set gas used.
func (b *Builder) GasUsed(used uint64) *Builder {
	b.headerBody.GasUsed = used
	return b
}

// Beneficiary set recipient of reward.
func (b *Builder) Beneficiary(addr meter.Address) *Builder {
	b.headerBody.Beneficiary = addr
	return b
}

// StateRoot set state root.
func (b *Builder) StateRoot(hash meter.Bytes32) *Builder {
	b.headerBody.StateRoot = hash
	return b
}

// ReceiptsRoot set receipts root.
func (b *Builder) ReceiptsRoot(hash meter.Bytes32) *Builder {
	b.headerBody.ReceiptsRoot = hash
	return b
}

// Transaction add a transaction.
func (b *Builder) Transaction(tx *tx.Transaction) *Builder {
	b.txs = append(b.txs, tx)
	return b
}

// Build build a block object.
func (b *Builder) Build() *Block {
	header := Header{Body: b.headerBody}
	header.Body.TxsRoot = b.txs.RootHash()

	return &Block{
		BlockHeader: &header,
		Txs:         b.txs,
	}
}
