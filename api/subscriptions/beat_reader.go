// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package subscriptions

import (
	"bytes"

	Block "github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/meter/bloom"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type beatReader struct {
	chain       *chain.Chain
	blockReader chain.BlockReader
}

func newBeatReader(chain *chain.Chain, position meter.Bytes32) *beatReader {
	return &beatReader{
		chain:       chain,
		blockReader: chain.NewBlockReader(position),
	}
}

func (br *beatReader) Read() ([]interface{}, bool, error) {
	blocks, err := br.blockReader.Read()
	if err != nil {
		return nil, false, err
	}
	var msgs []interface{}
	for _, block := range blocks {
		header := block.Header()
		receipts, err := br.chain.GetBlockReceipts(header.ID())
		if err != nil {
			return nil, false, err
		}
		txs := block.Transactions()
		bloomContent := &bloomContent{}
		for i, receipt := range receipts {
			for _, output := range receipt.Outputs {
				for _, event := range output.Events {
					bloomContent.add(event.Address.Bytes())
					for _, topic := range event.Topics {
						bloomContent.add(topic.Bytes())
					}
				}
				for _, transfer := range output.Transfers {
					bloomContent.add(transfer.Sender.Bytes())
					bloomContent.add(transfer.Recipient.Bytes())
				}
			}
			origin, _ := txs[i].Signer()
			bloomContent.add(origin.Bytes())
		}
		signer, _ := header.Signer()
		bloomContent.add(signer.Bytes())
		bloomContent.add(header.Beneficiary().Bytes())

		k := bloom.LegacyEstimateBloomK(bloomContent.len())
		bloom := bloom.NewLegacyBloom(k)
		for _, item := range bloomContent.items {
			bloom.Add(item)
		}

		var epoch uint64
		isKBlock := (header.BlockType() == Block.BLOCK_TYPE_K_BLOCK)
		if isKBlock {
			epoch = block.QC.EpochID
		} else if len(block.CommitteeInfos.CommitteeInfo) > 0 {
			epoch = block.CommitteeInfos.Epoch
		} else {
			epoch = block.QC.EpochID
		}

		msgs = append(msgs, &BeatMessage{
			ID:           header.ID(),
			ParentID:     header.ParentID(),
			UncleHash:    meter.Bytes32{},
			Signer:       signer,
			Beneficiary:  header.Beneficiary(),
			StateRoot:    header.StateRoot(),
			TxsRoot:      header.TxsRoot(),
			ReceiptsRoot: header.ReceiptsRoot(),
			Bloom:        hexutil.Encode(bloom.Bits[:]),
			K:            uint32(k),
			Difficaulty:  hexutil.EncodeUint64(uint64(0)),
			Number:       hexutil.EncodeUint64(uint64(header.Number())),
			Timestamp:    header.Timestamp(),
			GasLimit:     header.GasLimit(),
			GasUsed:      header.GasUsed(),
			Extra:        hexutil.Encode([]byte{}),
			Nonce:        0,
			Epoch:        epoch,
		})
	}
	return msgs, len(blocks) > 0, nil
}

type bloomContent struct {
	items [][]byte
}

func (bc *bloomContent) add(item []byte) {
	bc.items = append(bc.items, bytes.TrimLeft(item, "\x00"))
}

func (bc *bloomContent) len() int {
	return len(bc.items)
}
