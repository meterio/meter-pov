// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package transferslegacy

import (
	"github.com/dfinlab/meter/api/transactions"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/common/math"
)

type FilteredTransfer struct {
	Sender    meter.Address         `json:"sender"`
	Recipient meter.Address         `json:"recipient"`
	Amount    *math.HexOrDecimal256 `json:"amount"`
	Token     uint32                `json:"token"`
	Meta      transactions.LogMeta  `json:"meta"`
}

func convertTransfer(transfer *logdb.Transfer) *FilteredTransfer {
	v := math.HexOrDecimal256(*transfer.Amount)
	return &FilteredTransfer{
		Sender:    transfer.Sender,
		Recipient: transfer.Recipient,
		Amount:    &v,
		Token:     transfer.Token,
		Meta: transactions.LogMeta{
			BlockID:        transfer.BlockID,
			BlockNumber:    transfer.BlockNumber,
			BlockTimestamp: transfer.BlockTime,
			TxID:           transfer.TxID,
			TxOrigin:       transfer.TxOrigin,
		},
	}
}

type AddressSet struct {
	TxOrigin  *meter.Address //who send transaction
	Sender    *meter.Address //who transferred tokens
	Recipient *meter.Address //who recieved tokens
}

type TransferFilter struct {
	TxID        *meter.Bytes32
	AddressSets []*AddressSet
	Range       *logdb.Range
	Options     *logdb.Options
	Order       logdb.Order //default asc
}

func convertTransferFilter(tf *TransferFilter) *logdb.TransferFilter {
	t := &logdb.TransferFilter{
		TxID:    tf.TxID,
		Range:   tf.Range,
		Options: tf.Options,
		Order:   tf.Order,
	}
	transferCriterias := make([]*logdb.TransferCriteria, len(tf.AddressSets))
	for i, addressSet := range tf.AddressSets {
		transferCriterias[i] = &logdb.TransferCriteria{
			TxOrigin:  addressSet.TxOrigin,
			Sender:    addressSet.Sender,
			Recipient: addressSet.Recipient,
		}
	}
	t.CriteriaSet = transferCriterias
	return t
}
