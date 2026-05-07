// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package ethapi

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
)

var (
	emptyUnclesHash = "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"
	zeroHash        = "0x0000000000000000000000000000000000000000000000000000000000000000"
	zeroBloom       = "0x" + hex.EncodeToString(make([]byte, 256))
	zeroNonce       = "0x0000000000000000"
)

func hexUint64(n uint64) string {
	return hexutil.EncodeUint64(n)
}

func hexBig(b *big.Int) string {
	return hexutil.EncodeBig(b)
}

func meterBlockToEthBlock(blk *block.Block, receipts tx.Receipts, expanded bool, chainID *big.Int, baseGasPrice *big.Int) map[string]interface{} {
	header := blk.Header()
	signer, _ := header.Signer()

	result := map[string]interface{}{
		"hash":             header.ID().String(),
		"parentHash":       header.ParentID().String(),
		"number":           hexUint64(uint64(header.Number())),
		"timestamp":        hexUint64(header.Timestamp()),
		"gasLimit":         hexUint64(header.GasLimit()),
		"gasUsed":          hexUint64(header.GasUsed()),
		"miner":            signer.String(),
		"totalDifficulty":  hexUint64(header.TotalScore()),
		"transactionsRoot": header.TxsRoot().String(),
		"stateRoot":        header.StateRoot().String(),
		"receiptsRoot":     header.ReceiptsRoot().String(),
		"size":             hexUint64(uint64(blk.Size())),
		"nonce":            zeroNonce,
		"mixHash":          zeroHash,
		"sha3Uncles":       emptyUnclesHash,
		"logsBloom":        zeroBloom,
		"extraData":        "0x",
		"difficulty":       "0x0",
		"uncles":           []string{},
		"baseFeePerGas":    hexBig(baseGasPrice),
	}

	if expanded {
		txs := make([]interface{}, 0, len(blk.Txs))
		for i, t := range blk.Txs {
			meta := &chain.TxMeta{
				BlockID: header.ID(),
				Index:   uint64(i),
			}
			var receipt *tx.Receipt
			if i < len(receipts) {
				receipt = receipts[i]
			}
			txObj := meterTxToEthTx(t, meta, header, chainID, baseGasPrice, receipt)
			txs = append(txs, txObj)
		}
		result["transactions"] = txs
	} else {
		txHashes := make([]string, 0, len(blk.Txs))
		for _, t := range blk.Txs {
			txHashes = append(txHashes, t.ID().String())
		}
		result["transactions"] = txHashes
	}

	return result
}

func meterTxToEthTx(t *tx.Transaction, meta *chain.TxMeta, header *block.Header, chainID *big.Int, baseGasPrice *big.Int, receipt *tx.Receipt) map[string]interface{} {
	origin, _ := t.Signer()
	clauses := t.Clauses()

	var to interface{}
	value := "0x0"
	input := "0x"

	if len(clauses) > 0 {
		c := clauses[0]
		if c.To() != nil {
			to = c.To().String()
		}
		if c.Value() != nil {
			value = hexBig(c.Value())
		}
		if len(c.Data()) > 0 {
			input = hexutil.Encode(c.Data())
		}
	}

	gasPrice := t.GasPrice(baseGasPrice)

	v := big.NewInt(0)
	r := big.NewInt(0)
	s := big.NewInt(0)
	txType := "0x0"

	if t.IsEthTx() {
		ethTx, err := t.GetEthTx()
		if err == nil {
			v, r, s = ethTx.RawSignatureValues()
			if ethTx.Type() == 2 {
				txType = "0x2"
			} else if ethTx.Type() == 1 {
				txType = "0x1"
			}
		}
	} else {
		sig := t.Signature()
		if len(sig) >= 65 {
			r.SetBytes(sig[:32])
			s.SetBytes(sig[32:64])
			v.SetBytes(sig[64:65])
		}
	}

	result := map[string]interface{}{
		"hash":                 t.ID().String(),
		"blockHash":            header.ID().String(),
		"blockNumber":          hexUint64(uint64(header.Number())),
		"transactionIndex":     hexUint64(meta.Index),
		"from":                 origin.String(),
		"to":                   to,
		"value":                value,
		"input":                input,
		"gas":                  hexUint64(t.Gas()),
		"gasPrice":             hexBig(gasPrice),
		"nonce":                hexUint64(t.Nonce()),
		"v":                    hexBig(v),
		"r":                    hexBig(r),
		"s":                    hexBig(s),
		"type":                 txType,
		"chainId":              hexBig(chainID),
		"maxPriorityFeePerGas": "0x0",
		"maxFeePerGas":         hexBig(gasPrice),
	}

	return result
}

func meterReceiptToEthReceipt(receipt *tx.Receipt, t *tx.Transaction, meta *chain.TxMeta, header *block.Header, chainID *big.Int, baseGasPrice *big.Int, cumulativeGasUsed uint64) map[string]interface{} {
	origin, _ := t.Signer()
	clauses := t.Clauses()

	var to interface{}
	if len(clauses) > 0 && clauses[0].To() != nil {
		to = clauses[0].To().String()
	}

	status := "0x1"
	if receipt.Reverted {
		status = "0x0"
	}

	logs := make([]interface{}, 0)
	logIndex := 0
	for _, output := range receipt.Outputs {
		for _, event := range output.Events {
			logs = append(logs, meterLogToEthLog(event, header, t, meta, logIndex))
			logIndex++
		}
	}

	gasPrice := t.GasPrice(baseGasPrice)

	var contractAddress interface{}
	if len(clauses) > 0 && clauses[0].To() == nil {
		nonce := t.Nonce()
		addr := meter.Address(meter.EthCreateContractAddress(common.Address(origin), uint32(nonce)))
		contractAddress = addr.String()
	}

	txType := "0x0"
	if t.IsEthTx() {
		if ethTx, err := t.GetEthTx(); err == nil {
			switch ethTx.Type() {
			case 2:
				txType = "0x2"
			case 1:
				txType = "0x1"
			}
		}
	}

	result := map[string]interface{}{
		"transactionHash":   t.ID().String(),
		"transactionIndex":  hexUint64(meta.Index),
		"blockHash":         header.ID().String(),
		"blockNumber":       hexUint64(uint64(header.Number())),
		"from":              origin.String(),
		"to":                to,
		"gasUsed":           hexUint64(receipt.GasUsed),
		"cumulativeGasUsed": hexUint64(cumulativeGasUsed),
		"contractAddress":   contractAddress,
		"logs":              logs,
		"logsBloom":         zeroBloom,
		"status":            status,
		"type":              txType,
		"effectiveGasPrice": hexBig(gasPrice),
	}

	return result
}

func meterLogToEthLog(event *tx.Event, header *block.Header, t *tx.Transaction, meta *chain.TxMeta, logIndex int) map[string]interface{} {
	topics := make([]string, 0, len(event.Topics))
	for _, topic := range event.Topics {
		topics = append(topics, topic.String())
	}

	return map[string]interface{}{
		"address":          event.Address.String(),
		"topics":           topics,
		"data":             hexutil.Encode(event.Data),
		"blockHash":        header.ID().String(),
		"blockNumber":      hexUint64(uint64(header.Number())),
		"transactionHash":  t.ID().String(),
		"transactionIndex": hexUint64(meta.Index),
		"logIndex":         hexUint64(uint64(logIndex)),
		"removed":          false,
	}
}
