// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package proto

import "fmt"

// Constants
const (
	Name              = "meter"
	Version    uint   = 1
	Length     uint64 = 11
	MaxMsgSize        = 2 * 1024 * 1024 // max size 2M bytes
)

// Protocol messages of meter
const (
	MsgGetStatus = iota
	MsgNewBlockID
	MsgNewBlock
	MsgNewTx
	MsgGetBlockByID
	MsgGetBlockIDByNumber
	MsgGetBlocksFromNumber // fetch blocks from given number (including given number)
	MsgGetTxs
	MsgNewPowBlock
)

// MsgName convert msg code to string.
func MsgName(msgCode uint64) string {
	switch msgCode {
	case MsgGetStatus:
		return "GetStatus"
	case MsgNewBlockID:
		return "NewBlockID"
	case MsgNewBlock:
		return "NewBlock"
	case MsgNewTx:
		return "NewTx"
	case MsgGetBlockByID:
		return "GetBlockByID"
	case MsgGetBlockIDByNumber:
		return "GetBlockIDByNumber"
	case MsgGetBlocksFromNumber:
		return "GetBlocksFromNumber"
	case MsgGetTxs:
		return "GetTxs"
	case MsgNewPowBlock:
		return "NewPowBlock"
	default:
		return fmt.Sprintf("unknown msg code(%v)", msgCode)
	}
}
