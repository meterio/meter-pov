// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package proto

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
