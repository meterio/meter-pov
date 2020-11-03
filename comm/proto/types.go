// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package proto

import (
	"context"
	"fmt"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"
)

type (
	// Status result of MsgGetStatus.
	Status struct {
		GenesisBlockID meter.Bytes32
		SysTimestamp   uint64
		BestBlockID    meter.Bytes32
		TotalScore     uint64
	}
)

type WireQC struct {
	Magic [4]byte
	QC    *block.QuorumCert
}

// RPC defines RPC interface.
type RPC interface {
	Notify(ctx context.Context, msgCode uint64, arg interface{}) error
	Call(ctx context.Context, msgCode uint64, arg interface{}, result interface{}) error
}

// GetStatus get status of remote peer.
func GetStatus(ctx context.Context, rpc RPC) (*Status, error) {
	var status Status
	if err := rpc.Call(ctx, MsgGetStatus, &struct{}{}, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// NotifyNewBlockID notify new block ID to remote peer.
func NotifyNewBlockID(ctx context.Context, rpc RPC, id meter.Bytes32) error {
	return rpc.Notify(ctx, MsgNewBlockID, &id)
}

// NotifyNewBlock notify new block to remote peer.
func NotifyNewBlock(ctx context.Context, rpc RPC, block *block.Block) error {
	return rpc.Notify(ctx, MsgNewBlock, block)
}

// NotifyNewTx notify new tx to remote peer.
func NotifyNewTx(ctx context.Context, rpc RPC, tx *tx.Transaction) error {
	return rpc.Notify(ctx, MsgNewTx, tx)
}

// NotifyNewPow notify new pow block to remote peer.
func NotifyNewPowBlock(ctx context.Context, rpc RPC, powBlockInfo *powpool.PowBlockInfo) error {
	return rpc.Notify(ctx, MsgNewPowBlock, powBlockInfo)
}

func NotifyNewBestQC(ctx context.Context, rpc RPC, qc *block.QuorumCert) error {
	return rpc.Notify(ctx, MsgNewBestQC, &WireQC{block.BlockMagicVersion1, qc})
}

// GetBlockByID query block from remote peer by given block ID.
// It may return nil block even no error.
func GetBlockByID(ctx context.Context, rpc RPC, id meter.Bytes32) (rlp.RawValue, error) {
	var result []rlp.RawValue
	if err := rpc.Call(ctx, MsgGetBlockByID, id, &result); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result[0], nil
}

// GetBlockIDByNumber query block ID from remote peer by given number.
func GetBlockIDByNumber(ctx context.Context, rpc RPC, num uint32) (meter.Bytes32, error) {
	var id meter.Bytes32
	if err := rpc.Call(ctx, MsgGetBlockIDByNumber, num, &id); err != nil {
		return meter.Bytes32{}, err
	}
	return id, nil
}

// GetBlocksFromNumber get a batch of blocks starts with num from remote peer.
func GetBlocksFromNumber(ctx context.Context, rpc RPC, num uint32) ([]rlp.RawValue, error) {
	var blocks []rlp.RawValue
	if err := rpc.Call(ctx, MsgGetBlocksFromNumber, num, &blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func GetBestQC(ctx context.Context, rpc RPC) (*block.QuorumCert, error) {
	var wireQC WireQC
	if err := rpc.Call(ctx, MsgGetBestQC, nil, &wireQC); err != nil {
		return nil, err
	}
	if wireQC.Magic != block.BlockMagicVersion1 {
		str := fmt.Sprintf("QC magic mismatch, has %v, expect %v", wireQC.Magic, block.BlockMagicVersion1)
		fmt.Printf("%s, QC: %s", str, wireQC.QC.String())
		return nil, errors.New(str)
	}
	return wireQC.QC, nil
}

// GetTxs get txs from remote peer.
func GetTxs(ctx context.Context, rpc RPC) (tx.Transactions, error) {
	var txs tx.Transactions
	if err := rpc.Call(ctx, MsgGetTxs, &struct{}{}, &txs); err != nil {
		return nil, err
	}
	return txs, nil
}
