// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package subscriptions

import (
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
)

type blockReader struct {
	chain       *chain.Chain
	blockReader chain.BlockReader
}

func newBlockReader(chain *chain.Chain, position meter.Bytes32) *blockReader {
	return &blockReader{
		chain:       chain,
		blockReader: chain.NewBlockReader(position),
	}
}

func (br *blockReader) Read() ([]interface{}, bool, error) {
	blocks, err := br.blockReader.Read()
	if err != nil {
		return nil, false, err
	}
	var msgs []interface{}
	for _, block := range blocks {
		msg, err := convertBlock(block)
		if err != nil {
			return nil, false, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, len(blocks) > 0, nil
}
