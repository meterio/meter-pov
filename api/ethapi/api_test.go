package ethapi

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

// ---------- CallArgs tests ----------

func TestCallArgs_GasLimit(t *testing.T) {
	// nil gas
	args := CallArgs{}
	assert.Equal(t, uint64(0), args.gasLimit())

	// with gas
	gas := hexutil.Uint64(50000)
	args = CallArgs{Gas: &gas}
	assert.Equal(t, uint64(50000), args.gasLimit())
}

func TestCallArgs_DataBytes(t *testing.T) {
	// nil data and input
	args := CallArgs{}
	assert.Nil(t, args.dataBytes())

	// with Data
	data := hexutil.Bytes([]byte{0x01, 0x02})
	args = CallArgs{Data: &data}
	assert.Equal(t, []byte{0x01, 0x02}, args.dataBytes())

	// with Input (takes priority)
	input := hexutil.Bytes([]byte{0x03, 0x04})
	args = CallArgs{Data: &data, Input: &input}
	assert.Equal(t, []byte{0x03, 0x04}, args.dataBytes())

	// only Input
	args = CallArgs{Input: &input}
	assert.Equal(t, []byte{0x03, 0x04}, args.dataBytes())
}

func TestCallArgs_DataBytes_Empty(t *testing.T) {
	data := hexutil.Bytes([]byte{})
	args := CallArgs{Data: &data}
	assert.Equal(t, []byte{}, args.dataBytes())
}

// ---------- LogFilterArgs JSON tests ----------

func TestLogFilterArgs_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"fromBlock": "0x1",
		"toBlock": "0x100",
		"address": ["0x0000000000000000000000000000000000000001"],
		"topics": [["0x0000000000000000000000000000000000000000000000000000000000000001"]]
	}`

	var filter LogFilterArgs
	err := json.Unmarshal([]byte(jsonStr), &filter)
	assert.NoError(t, err)
	assert.NotNil(t, filter.FromBlock)
	assert.Equal(t, "0x1", *filter.FromBlock)
	assert.NotNil(t, filter.ToBlock)
	assert.Equal(t, "0x100", *filter.ToBlock)
	assert.Equal(t, 1, len(filter.Addresses))
	assert.Equal(t, 1, len(filter.Topics))
}

func TestLogFilterArgs_JSONUnmarshal_Minimal(t *testing.T) {
	jsonStr := `{}`
	var filter LogFilterArgs
	err := json.Unmarshal([]byte(jsonStr), &filter)
	assert.NoError(t, err)
	assert.Nil(t, filter.FromBlock)
	assert.Nil(t, filter.ToBlock)
	assert.Nil(t, filter.Addresses)
	assert.Nil(t, filter.Topics)
}

// ---------- NetAPI tests ----------

func TestNetAPI_Version(t *testing.T) {
	api := &NetAPI{chainID: big.NewInt(82)}
	assert.Equal(t, "82", api.Version())

	api = &NetAPI{chainID: big.NewInt(83)}
	assert.Equal(t, "83", api.Version())
}

func TestNetAPI_Listening(t *testing.T) {
	api := &NetAPI{chainID: big.NewInt(82)}
	assert.False(t, api.Listening())
}

func TestNetAPI_PeerCount(t *testing.T) {
	api := &NetAPI{chainID: big.NewInt(82)}
	assert.Equal(t, "0x0", api.PeerCount())
}

// ---------- Web3API tests ----------

func TestWeb3API_ClientVersion(t *testing.T) {
	api := &Web3API{}
	v := api.ClientVersion()
	assert.Contains(t, v, "meter-pov")
}

// ---------- RPCAPI tests ----------

func TestRPCAPI_Modules(t *testing.T) {
	api := &RPCAPI{}
	modules := api.Modules()
	assert.Equal(t, "1.0", modules["eth"])
	assert.Equal(t, "1.0", modules["net"])
	assert.Equal(t, "1.0", modules["web3"])
	assert.Equal(t, 3, len(modules))
}

// ---------- EVMAPI tests ----------

func TestEVMAPI_Snapshot(t *testing.T) {
	api := &EVMAPI{}
	assert.Equal(t, "0x0", api.Snapshot())
}

func TestEVMAPI_Revert(t *testing.T) {
	api := &EVMAPI{}
	assert.True(t, api.Revert("0x1"))
	assert.True(t, api.Revert("anything"))
}

// ---------- EthAPI static method tests ----------

func TestEthAPI_ChainId(t *testing.T) {
	api := &EthAPI{chainID: big.NewInt(82)}
	assert.Equal(t, "0x52", api.ChainId())

	api = &EthAPI{chainID: big.NewInt(83)}
	assert.Equal(t, "0x53", api.ChainId())
}

func TestEthAPI_MaxPriorityFeePerGas(t *testing.T) {
	api := &EthAPI{}
	assert.Equal(t, "0x0", api.MaxPriorityFeePerGas())
}

func TestEthAPI_Syncing(t *testing.T) {
	api := &EthAPI{}
	result, err := api.Syncing()
	assert.NoError(t, err)
	assert.Equal(t, false, result)
}

func TestEthAPI_GetTransactionCount(t *testing.T) {
	api := &EthAPI{}
	addr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")

	result1 := api.GetTransactionCount(addr, "latest")
	result2 := api.GetTransactionCount(addr, "latest")

	// Should return hex strings
	assert.Equal(t, "0x", result1[:2])
	assert.Equal(t, "0x", result2[:2])
	// Random, so they should almost certainly differ (1 in 2^64 chance of collision)
	// Skip equality check as it's probabilistic
}

// ---------- Filter management tests ----------

func TestEthAPI_FilterLifecycle(t *testing.T) {
	// We can't use NewBlockFilter without a chain, but we can test UninstallFilter
	api := &EthAPI{
		filters: make(map[string]*blockFilter),
	}

	// Manually insert a filter
	api.filters["0x123"] = &blockFilter{lastBlock: 100}

	// UninstallFilter existing
	assert.True(t, api.UninstallFilter("0x123"))

	// UninstallFilter non-existing
	assert.False(t, api.UninstallFilter("0x123"))
	assert.False(t, api.UninstallFilter("0xnonexistent"))
}

func TestEthAPI_GetFilterChanges_NotFound(t *testing.T) {
	api := &EthAPI{
		filters: make(map[string]*blockFilter),
	}

	hashes, err := api.GetFilterChanges("0xnonexistent")
	assert.Error(t, err)
	assert.Equal(t, []string{}, hashes)
}

// ---------- buildCriteriaSet tests ----------

func TestBuildCriteriaSet(t *testing.T) {
	t.Run("no addresses no topics", func(t *testing.T) {
		cs := buildCriteriaSet(nil, nil)
		assert.Equal(t, 1, len(cs))
		assert.Nil(t, cs[0].Address)
		for _, topic := range cs[0].Topics {
			assert.Nil(t, topic)
		}
	})

	t.Run("single address no topics", func(t *testing.T) {
		addr := common.HexToAddress("0xdeadbeef")
		cs := buildCriteriaSet([]common.Address{addr}, nil)
		assert.Equal(t, 1, len(cs))
		assert.NotNil(t, cs[0].Address)
	})

	t.Run("single topic exact match", func(t *testing.T) {
		h := common.HexToHash("0x1234")
		cs := buildCriteriaSet(nil, [][]common.Hash{{h}})
		assert.Equal(t, 1, len(cs))
		assert.NotNil(t, cs[0].Topics[0])
		assert.Equal(t, meter.Bytes32(h), *cs[0].Topics[0])
	})

	t.Run("OR topics expand into multiple criteria", func(t *testing.T) {
		h1 := common.HexToHash("0x1111")
		h2 := common.HexToHash("0x2222")
		cs := buildCriteriaSet(nil, [][]common.Hash{{h1, h2}})
		assert.Equal(t, 2, len(cs))
		assert.Equal(t, meter.Bytes32(h1), *cs[0].Topics[0])
		assert.Equal(t, meter.Bytes32(h2), *cs[1].Topics[0])
	})

	t.Run("two addresses times two OR topics", func(t *testing.T) {
		a1 := common.HexToAddress("0x1111")
		a2 := common.HexToAddress("0x2222")
		h1 := common.HexToHash("0xaaaa")
		h2 := common.HexToHash("0xbbbb")
		cs := buildCriteriaSet([]common.Address{a1, a2}, [][]common.Hash{{h1, h2}})
		// 2 addresses × 2 topic OR values = 4 criteria
		assert.Equal(t, 4, len(cs))
	})

	t.Run("empty topic slot means match any", func(t *testing.T) {
		h1 := common.HexToHash("0xdddd")
		cs := buildCriteriaSet(nil, [][]common.Hash{{}, {h1}})
		assert.Equal(t, 1, len(cs))
		assert.Nil(t, cs[0].Topics[0])
		assert.NotNil(t, cs[0].Topics[1])
	})

	t.Run("more than 5 topic positions truncated", func(t *testing.T) {
		topics := make([][]common.Hash, 7)
		for i := range topics {
			topics[i] = []common.Hash{common.HexToHash("0x01")}
		}
		cs := buildCriteriaSet(nil, topics)
		assert.Equal(t, 1, len(cs))
		for i := 0; i < 5; i++ {
			assert.NotNil(t, cs[0].Topics[i])
		}
	})
}

// ---------- blockNumberToUint32 tests (needs chain mock, test parsing only) ----------

func TestBlockNumberToUint32_Parsing(t *testing.T) {
	// We test the format parsing logic through the public method signature expectations.
	// The "earliest" case returns 0 and doesn't need chain.
	// Other cases need chain, so we only test format validation here.

	// Test that hexUint64 parsing would work for valid inputs
	n, err := hexutil.DecodeUint64("0x10")
	assert.NoError(t, err)
	assert.Equal(t, uint64(16), n)

	n, err = hexutil.DecodeUint64("0x0")
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), n)

	_, err = hexutil.DecodeUint64("invalid")
	assert.Error(t, err)

	_, err = hexutil.DecodeUint64("latest")
	assert.Error(t, err)
}

// ---------- CallArgs JSON marshaling ----------

func TestCallArgs_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"from": "0x0000000000000000000000000000000000000001",
		"to": "0x0000000000000000000000000000000000000002",
		"gas": "0x5208",
		"gasPrice": "0x174876e800",
		"value": "0x3e8",
		"data": "0xabcd"
	}`

	var args CallArgs
	err := json.Unmarshal([]byte(jsonStr), &args)
	assert.NoError(t, err)
	assert.NotNil(t, args.From)
	assert.NotNil(t, args.To)
	assert.NotNil(t, args.Gas)
	assert.Equal(t, uint64(21000), args.gasLimit())
	assert.NotNil(t, args.GasPrice)
	assert.NotNil(t, args.Value)
	assert.NotNil(t, args.Data)
	assert.Equal(t, []byte{0xab, 0xcd}, args.dataBytes())
}

func TestCallArgs_JSONUnmarshal_Minimal(t *testing.T) {
	jsonStr := `{}`
	var args CallArgs
	err := json.Unmarshal([]byte(jsonStr), &args)
	assert.NoError(t, err)
	assert.Nil(t, args.From)
	assert.Nil(t, args.To)
	assert.Nil(t, args.Gas)
	assert.Equal(t, uint64(0), args.gasLimit())
	assert.Nil(t, args.dataBytes())
}

func TestCallArgs_JSONUnmarshal_InputOverridesData(t *testing.T) {
	jsonStr := `{
		"data": "0x1111",
		"input": "0x2222"
	}`
	var args CallArgs
	err := json.Unmarshal([]byte(jsonStr), &args)
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x22, 0x22}, args.dataBytes())
}
