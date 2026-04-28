package ethapi

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func TestHexUint64(t *testing.T) {
	assert.Equal(t, "0x0", hexUint64(0))
	assert.Equal(t, "0x1", hexUint64(1))
	assert.Equal(t, "0xff", hexUint64(255))
	assert.Equal(t, "0x100", hexUint64(256))
	assert.Equal(t, "0xffffffffffffffff", hexUint64(^uint64(0)))
}

func TestHexBig(t *testing.T) {
	assert.Equal(t, "0x0", hexBig(big.NewInt(0)))
	assert.Equal(t, "0x1", hexBig(big.NewInt(1)))
	assert.Equal(t, "0xa", hexBig(big.NewInt(10)))
	assert.Equal(t, "0x64", hexBig(big.NewInt(100)))

	large := new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil)
	result := hexBig(large)
	assert.NotEmpty(t, result)
	assert.Equal(t, "0x", result[:2])
}

func buildTestBlock(txs ...*tx.Transaction) *block.Block {
	builder := new(block.Builder).
		Timestamp(1000000).
		GasLimit(10000000).
		GasUsed(21000).
		TotalScore(100).
		Beneficiary(meter.BytesToAddress([]byte("beneficiary")))

	for _, t := range txs {
		builder = builder.Transaction(t)
	}
	return builder.Build()
}

func buildSignedTestTx() *tx.Transaction {
	toAddr := meter.BytesToAddress([]byte("recipient"))
	clause := tx.NewClause(&toAddr).
		WithValue(big.NewInt(1000)).
		WithData([]byte{0xab, 0xcd})

	trx := new(tx.Builder).
		ChainTag(82).
		Gas(21000).
		GasPriceCoef(0).
		Nonce(12345).
		Expiration(720).
		Clause(clause).
		Build()

	key, _ := crypto.GenerateKey()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	return trx.WithSignature(sig)
}

func buildContractCreationTx() *tx.Transaction {
	clause := tx.NewClause(nil).
		WithValue(big.NewInt(0)).
		WithData([]byte{0x60, 0x60, 0x60, 0x40})

	trx := new(tx.Builder).
		ChainTag(82).
		Gas(100000).
		GasPriceCoef(0).
		Nonce(99).
		Expiration(720).
		Clause(clause).
		Build()

	key, _ := crypto.GenerateKey()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	return trx.WithSignature(sig)
}

func TestMeterBlockToEthBlock_CollapsedTxs(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	chainID := big.NewInt(82)
	baseGasPrice := big.NewInt(500000000000)

	result := meterBlockToEthBlock(blk, nil, false, chainID, baseGasPrice)

	// Check required Ethereum block fields
	assert.Equal(t, blk.Header().ID().String(), result["hash"])
	assert.Equal(t, blk.Header().ParentID().String(), result["parentHash"])
	assert.Equal(t, hexUint64(uint64(blk.Header().Number())), result["number"])
	assert.Equal(t, hexUint64(blk.Header().Timestamp()), result["timestamp"])
	assert.Equal(t, hexUint64(blk.Header().GasLimit()), result["gasLimit"])
	assert.Equal(t, hexUint64(blk.Header().GasUsed()), result["gasUsed"])
	assert.Equal(t, hexUint64(blk.Header().TotalScore()), result["totalDifficulty"])

	// Fake ETH fields
	assert.Equal(t, zeroNonce, result["nonce"])
	assert.Equal(t, zeroHash, result["mixHash"])
	assert.Equal(t, emptyUnclesHash, result["sha3Uncles"])
	assert.Equal(t, zeroBloom, result["logsBloom"])
	assert.Equal(t, "0x", result["extraData"])
	assert.Equal(t, "0x0", result["difficulty"])
	assert.Equal(t, "0x0", result["baseFeePerGas"])
	assert.Equal(t, []string{}, result["uncles"])

	// Collapsed transactions - should be a list of hashes
	txHashes, ok := result["transactions"].([]string)
	assert.True(t, ok, "transactions should be []string for collapsed mode")
	assert.Equal(t, 1, len(txHashes))
	assert.Equal(t, trx.ID().String(), txHashes[0])
}

func TestMeterBlockToEthBlock_ExpandedTxs(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	chainID := big.NewInt(82)
	baseGasPrice := big.NewInt(500000000000)

	receipt := &tx.Receipt{
		GasUsed:  21000,
		GasPayer: meter.BytesToAddress([]byte("payer")),
		Paid:     big.NewInt(100),
		Reward:   big.NewInt(10),
		Reverted: false,
		Outputs: []*tx.Output{
			{Events: tx.Events{}, Transfers: tx.Transfers{}},
		},
	}
	receipts := tx.Receipts{receipt}

	result := meterBlockToEthBlock(blk, receipts, true, chainID, baseGasPrice)

	txs, ok := result["transactions"].([]interface{})
	assert.True(t, ok, "transactions should be []interface{} for expanded mode")
	assert.Equal(t, 1, len(txs))

	txObj, ok := txs[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, trx.ID().String(), txObj["hash"])
	assert.Equal(t, hexUint64(0), txObj["transactionIndex"])
}

func TestMeterBlockToEthBlock_EmptyBlock(t *testing.T) {
	blk := buildTestBlock()
	chainID := big.NewInt(82)
	baseGasPrice := big.NewInt(500000000000)

	result := meterBlockToEthBlock(blk, nil, false, chainID, baseGasPrice)
	txHashes, ok := result["transactions"].([]string)
	assert.True(t, ok)
	assert.Equal(t, 0, len(txHashes))
}

func TestMeterTxToEthTx(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	chainID := big.NewInt(82)
	baseGasPrice := big.NewInt(500000000000)
	meta := &chain.TxMeta{
		BlockID: header.ID(),
		Index:   0,
	}

	result := meterTxToEthTx(trx, meta, header, chainID, baseGasPrice, nil)

	assert.Equal(t, trx.ID().String(), result["hash"])
	assert.Equal(t, header.ID().String(), result["blockHash"])
	assert.Equal(t, hexUint64(uint64(header.Number())), result["blockNumber"])
	assert.Equal(t, hexUint64(0), result["transactionIndex"])
	assert.Equal(t, hexUint64(21000), result["gas"])
	assert.Equal(t, hexUint64(12345), result["nonce"])
	assert.Equal(t, "0x0", result["type"])
	assert.Equal(t, hexBig(chainID), result["chainId"])
	assert.Equal(t, "0x0", result["maxPriorityFeePerGas"])

	// Check to address is set
	assert.NotNil(t, result["to"])

	// Check from is set
	from, ok := result["from"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, from)

	// Check value and input are set
	assert.NotEmpty(t, result["value"])
	assert.NotEmpty(t, result["input"])

	// Check signature fields exist
	assert.NotNil(t, result["v"])
	assert.NotNil(t, result["r"])
	assert.NotNil(t, result["s"])
}

func TestMeterTxToEthTx_ContractCreation(t *testing.T) {
	trx := buildContractCreationTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	chainID := big.NewInt(82)
	baseGasPrice := big.NewInt(500000000000)
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 0}

	result := meterTxToEthTx(trx, meta, header, chainID, baseGasPrice, nil)

	// For contract creation, "to" should be nil
	assert.Nil(t, result["to"])
	assert.NotEmpty(t, result["input"])
}

func TestMeterTxToEthTx_NoClauses(t *testing.T) {
	// Transaction with no clauses
	trx := new(tx.Builder).
		ChainTag(82).
		Gas(21000).
		Nonce(1).
		Build()
	key, _ := crypto.GenerateKey()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)

	blk := buildTestBlock(trx)
	header := blk.Header()
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 0}

	result := meterTxToEthTx(trx, meta, header, big.NewInt(82), big.NewInt(0), nil)

	// With no clauses, to should be nil, value should be "0x0", input should be "0x"
	assert.Nil(t, result["to"])
	assert.Equal(t, "0x0", result["value"])
	assert.Equal(t, "0x", result["input"])
}

func TestMeterReceiptToEthReceipt_Success(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	chainID := big.NewInt(82)
	baseGasPrice := big.NewInt(500000000000)
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 0}

	topic1 := meter.BytesToBytes32([]byte("Transfer(address,address,uint256)"))
	receipt := &tx.Receipt{
		GasUsed:  21000,
		GasPayer: meter.BytesToAddress([]byte("payer")),
		Paid:     big.NewInt(100),
		Reward:   big.NewInt(10),
		Reverted: false,
		Outputs: []*tx.Output{
			{
				Events: tx.Events{
					&tx.Event{
						Address: meter.BytesToAddress([]byte("contract")),
						Topics:  []meter.Bytes32{topic1},
						Data:    []byte{0x01, 0x02, 0x03},
					},
				},
				Transfers: tx.Transfers{},
			},
		},
	}

	result := meterReceiptToEthReceipt(receipt, trx, meta, header, chainID, baseGasPrice)

	assert.Equal(t, trx.ID().String(), result["transactionHash"])
	assert.Equal(t, header.ID().String(), result["blockHash"])
	assert.Equal(t, hexUint64(uint64(header.Number())), result["blockNumber"])
	assert.Equal(t, hexUint64(0), result["transactionIndex"])
	assert.Equal(t, hexUint64(21000), result["gasUsed"])
	assert.Equal(t, hexUint64(21000), result["cumulativeGasUsed"])
	assert.Equal(t, "0x1", result["status"]) // success
	assert.Equal(t, "0x0", result["type"])
	assert.Equal(t, zeroBloom, result["logsBloom"])
	assert.Nil(t, result["contractAddress"]) // not a contract creation

	// Check logs
	logs, ok := result["logs"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, len(logs))

	log, ok := logs[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, meter.BytesToAddress([]byte("contract")).String(), log["address"])
	assert.Equal(t, hexUint64(0), log["logIndex"])
	assert.Equal(t, false, log["removed"])
}

func TestMeterReceiptToEthReceipt_Reverted(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 0}

	receipt := &tx.Receipt{
		GasUsed:  21000,
		GasPayer: meter.BytesToAddress([]byte("payer")),
		Paid:     big.NewInt(100),
		Reward:   big.NewInt(10),
		Reverted: true,
		Outputs:  []*tx.Output{},
	}

	result := meterReceiptToEthReceipt(receipt, trx, meta, header, big.NewInt(82), big.NewInt(0))
	assert.Equal(t, "0x0", result["status"]) // reverted
}

func TestMeterReceiptToEthReceipt_ContractCreation(t *testing.T) {
	trx := buildContractCreationTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 0}

	receipt := &tx.Receipt{
		GasUsed:  50000,
		GasPayer: meter.BytesToAddress([]byte("payer")),
		Paid:     big.NewInt(100),
		Reward:   big.NewInt(10),
		Reverted: false,
		Outputs:  []*tx.Output{},
	}

	result := meterReceiptToEthReceipt(receipt, trx, meta, header, big.NewInt(82), big.NewInt(0))

	// Contract creation should produce a contract address
	assert.NotNil(t, result["contractAddress"])
	addr, ok := result["contractAddress"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, addr)
	assert.Nil(t, result["to"]) // to is nil for contract creation
}

func TestMeterReceiptToEthReceipt_MultipleEventsAcrossOutputs(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 2}

	topic1 := meter.BytesToBytes32([]byte("event1"))
	topic2 := meter.BytesToBytes32([]byte("event2"))
	topic3 := meter.BytesToBytes32([]byte("event3"))

	receipt := &tx.Receipt{
		GasUsed:  42000,
		GasPayer: meter.BytesToAddress([]byte("payer")),
		Paid:     big.NewInt(100),
		Reward:   big.NewInt(10),
		Reverted: false,
		Outputs: []*tx.Output{
			{
				Events: tx.Events{
					&tx.Event{Address: meter.BytesToAddress([]byte("c1")), Topics: []meter.Bytes32{topic1}, Data: []byte{0x01}},
					&tx.Event{Address: meter.BytesToAddress([]byte("c2")), Topics: []meter.Bytes32{topic2}, Data: []byte{0x02}},
				},
			},
			{
				Events: tx.Events{
					&tx.Event{Address: meter.BytesToAddress([]byte("c3")), Topics: []meter.Bytes32{topic3}, Data: []byte{0x03}},
				},
			},
		},
	}

	result := meterReceiptToEthReceipt(receipt, trx, meta, header, big.NewInt(82), big.NewInt(0))

	logs, ok := result["logs"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 3, len(logs))

	// Verify log indices are sequential
	for i, l := range logs {
		logMap := l.(map[string]interface{})
		assert.Equal(t, hexUint64(uint64(i)), logMap["logIndex"])
		assert.Equal(t, hexUint64(2), logMap["transactionIndex"])
	}
}

func TestMeterLogToEthLog(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 3}

	topic1 := meter.BytesToBytes32([]byte("Transfer(address,address,uint256)"))
	topic2 := meter.BytesToBytes32([]byte("from_address"))
	event := &tx.Event{
		Address: meter.BytesToAddress([]byte("contractAddr")),
		Topics:  []meter.Bytes32{topic1, topic2},
		Data:    []byte{0xde, 0xad, 0xbe, 0xef},
	}

	result := meterLogToEthLog(event, header, trx, meta, 7)

	assert.Equal(t, event.Address.String(), result["address"])
	assert.Equal(t, header.ID().String(), result["blockHash"])
	assert.Equal(t, hexUint64(uint64(header.Number())), result["blockNumber"])
	assert.Equal(t, trx.ID().String(), result["transactionHash"])
	assert.Equal(t, hexUint64(3), result["transactionIndex"])
	assert.Equal(t, hexUint64(7), result["logIndex"])
	assert.Equal(t, false, result["removed"])

	topics, ok := result["topics"].([]string)
	assert.True(t, ok)
	assert.Equal(t, 2, len(topics))
	assert.Equal(t, topic1.String(), topics[0])
	assert.Equal(t, topic2.String(), topics[1])

	assert.Equal(t, "0xdeadbeef", result["data"])
}

func TestMeterLogToEthLog_NoTopics(t *testing.T) {
	trx := buildSignedTestTx()
	blk := buildTestBlock(trx)
	header := blk.Header()
	meta := &chain.TxMeta{BlockID: header.ID(), Index: 0}

	event := &tx.Event{
		Address: meter.BytesToAddress([]byte("contract")),
		Topics:  []meter.Bytes32{},
		Data:    []byte{},
	}

	result := meterLogToEthLog(event, header, trx, meta, 0)
	topics, ok := result["topics"].([]string)
	assert.True(t, ok)
	assert.Equal(t, 0, len(topics))
}

func TestConstants(t *testing.T) {
	// Verify constant values match Ethereum expectations
	assert.Equal(t, "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347", emptyUnclesHash)
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", zeroHash)
	assert.Equal(t, "0x0000000000000000", zeroNonce)
	assert.Equal(t, 514, len(zeroBloom)) // "0x" + 512 hex chars
	assert.Equal(t, "0x", zeroBloom[:2])
}
