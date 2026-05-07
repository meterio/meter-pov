// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package ethapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"math/rand"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
	"github.com/meterio/meter-pov/xenv"
)

// EthAPI implements the eth_* JSON-RPC methods.
type EthAPI struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	txPool       *txpool.TxPool
	logDB        *logdb.LogDB
	chainID      *big.Int
	callGasLimit uint64
	logger       *slog.Logger

	mu      sync.Mutex
	filters map[string]*blockFilter
}

type blockFilter struct {
	lastBlock uint32
}

func NewEthAPI(chain *chain.Chain, stateCreator *state.Creator, txPool *txpool.TxPool, logDB *logdb.LogDB, chainID *big.Int, callGasLimit uint64) *EthAPI {
	return &EthAPI{
		chain:        chain,
		stateCreator: stateCreator,
		txPool:       txPool,
		logDB:        logDB,
		chainID:      chainID,
		callGasLimit: callGasLimit,
		logger:       slog.With("api", "eth-rpc"),
		filters:      make(map[string]*blockFilter),
	}
}

// ---------- Trivial / Static ----------

func (api *EthAPI) ChainId() string {
	return hexBig(api.chainID)
}

func (api *EthAPI) GasPrice() (string, error) {
	best := api.chain.BestBlock()
	s, err := api.stateCreator.NewState(best.StateRoot())
	if err != nil {
		return "0x0", err
	}
	baseGasPrice := builtin.Params.Native(s).Get(meter.KeyBaseGasPrice)
	return hexBig(baseGasPrice), nil
}

func (api *EthAPI) MaxPriorityFeePerGas() string {
	return "0x0"
}

func (api *EthAPI) Syncing() (interface{}, error) {
	return false, nil
}

// ---------- Simple ----------

func (api *EthAPI) BlockNumber() string {
	return hexUint64(uint64(api.chain.BestBlock().Number()))
}

func (api *EthAPI) GetBalance(addr common.Address, blockNrOrHash string) (string, error) {
	header, err := api.resolveBlockNumber(blockNrOrHash)
	if err != nil {
		return "0x0", err
	}
	s, err := api.stateCreator.NewState(header.StateRoot())
	if err != nil {
		return "0x0", err
	}
	balance := s.GetBalance(meter.Address(addr))
	if err := s.Err(); err != nil {
		return "0x0", err
	}
	return hexBig(balance), nil
}

func (api *EthAPI) GetCode(addr common.Address, blockNrOrHash string) (string, error) {
	header, err := api.resolveBlockNumber(blockNrOrHash)
	if err != nil {
		return "0x", err
	}
	s, err := api.stateCreator.NewState(header.StateRoot())
	if err != nil {
		return "0x", err
	}
	code := s.GetCode(meter.Address(addr))
	if err := s.Err(); err != nil {
		return "0x", err
	}
	return hexutil.Encode(code), nil
}

func (api *EthAPI) GetStorageAt(addr common.Address, key string, blockNrOrHash string) (string, error) {
	header, err := api.resolveBlockNumber(blockNrOrHash)
	if err != nil {
		return "0x0", err
	}
	k, err := meter.ParseBytes32(key)
	if err != nil {
		return "0x0", fmt.Errorf("invalid storage key: %v", err)
	}
	s, err := api.stateCreator.NewState(header.StateRoot())
	if err != nil {
		return "0x0", err
	}
	storage := s.GetStorage(meter.Address(addr), k)
	if err := s.Err(); err != nil {
		return "0x0", err
	}
	return storage.String(), nil
}

func (api *EthAPI) GetTransactionCount(addr common.Address, blockNrOrHash string) string {
	// Meter doesn't track nonces like Ethereum. Return random nonce (matches gear behavior).
	return hexUint64(rand.Uint64())
}

func (api *EthAPI) NewBlockFilter() string {
	api.mu.Lock()
	defer api.mu.Unlock()
	id := fmt.Sprintf("0x%x", rand.Int63())
	api.filters[id] = &blockFilter{lastBlock: api.chain.BestBlock().Number()}
	return id
}

func (api *EthAPI) UninstallFilter(id string) bool {
	api.mu.Lock()
	defer api.mu.Unlock()
	_, ok := api.filters[id]
	delete(api.filters, id)
	return ok
}

func (api *EthAPI) GetFilterChanges(id string) ([]string, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	f, ok := api.filters[id]
	if !ok {
		return []string{}, fmt.Errorf("filter not found")
	}
	best := api.chain.BestBlock().Number()
	var hashes []string
	for i := f.lastBlock + 1; i <= best; i++ {
		blk, err := api.chain.GetTrunkBlock(i)
		if err != nil {
			break
		}
		hashes = append(hashes, blk.ID().String())
	}
	f.lastBlock = best
	return hashes, nil
}

// ---------- Medium ----------

func (api *EthAPI) GetBlockByNumber(blockNr string, fullTx bool) (interface{}, error) {
	header, err := api.resolveBlockNumber(blockNr)
	if err != nil {
		return nil, nil
	}
	blk, err := api.chain.GetBlock(header.ID())
	if err != nil {
		return nil, nil
	}
	return api.buildEthBlock(blk, fullTx)
}

func (api *EthAPI) GetBlockByHash(hash common.Hash, fullTx bool) (interface{}, error) {
	blockID := meter.Bytes32(hash)
	blk, err := api.chain.GetBlock(blockID)
	if err != nil {
		return nil, nil
	}
	return api.buildEthBlock(blk, fullTx)
}

func (api *EthAPI) GetTransactionByHash(hash common.Hash) (interface{}, error) {
	txID := meter.Bytes32(hash)
	t, txMeta, err := api.chain.GetTrunkTransaction(txID)
	if err != nil {
		if api.chain.IsNotFound(err) {
			if pending := api.txPool.Get(txID); pending != nil {
				return api.buildPendingTx(pending), nil
			}
			return nil, nil
		}
		return nil, err
	}
	header, err := api.chain.GetBlockHeader(txMeta.BlockID)
	if err != nil {
		return nil, err
	}
	baseGasPrice := api.getBaseGasPrice(header)
	return meterTxToEthTx(t, txMeta, header, api.chainID, baseGasPrice, nil), nil
}

func (api *EthAPI) GetTransactionReceipt(hash common.Hash) (interface{}, error) {
	txID := meter.Bytes32(hash)
	t, txMeta, err := api.chain.GetTrunkTransaction(txID)
	if err != nil {
		if api.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	header, err := api.chain.GetBlockHeader(txMeta.BlockID)
	if err != nil {
		return nil, err
	}
	best := api.chain.BestBlock()
	if header.Number() > best.Number() {
		return nil, nil
	}
	receipt, err := api.chain.GetTransactionReceipt(txMeta.BlockID, txMeta.Index)
	if err != nil {
		return nil, err
	}
	// compute cumulativeGasUsed by summing all receipts up to this tx
	allReceipts, _ := api.chain.GetBlockReceipts(txMeta.BlockID)
	cumGasUsed := uint64(0)
	for i, r := range allReceipts {
		cumGasUsed += r.GasUsed
		if uint64(i) == txMeta.Index {
			break
		}
	}
	baseGasPrice := api.getBaseGasPrice(header)
	return meterReceiptToEthReceipt(receipt, t, txMeta, header, api.chainID, baseGasPrice, cumGasUsed), nil
}

func (api *EthAPI) GetTransactionByBlockNumberAndIndex(blockNr string, index hexutil.Uint) (interface{}, error) {
	header, err := api.resolveBlockNumber(blockNr)
	if err != nil {
		return nil, nil
	}
	blk, err := api.chain.GetBlock(header.ID())
	if err != nil {
		return nil, nil
	}
	txs := blk.Txs
	if int(index) >= len(txs) {
		return nil, nil
	}
	t := txs[int(index)]
	meta := &chain.TxMeta{BlockID: header.ID(), Index: uint64(index)}
	baseGasPrice := api.getBaseGasPrice(header)
	return meterTxToEthTx(t, meta, header, api.chainID, baseGasPrice, nil), nil
}

func (api *EthAPI) GetTransactionByBlockHashAndIndex(hash common.Hash, index hexutil.Uint) (interface{}, error) {
	blockID := meter.Bytes32(hash)
	blk, err := api.chain.GetBlock(blockID)
	if err != nil {
		return nil, nil
	}
	header := blk.Header()
	txs := blk.Txs
	if int(index) >= len(txs) {
		return nil, nil
	}
	t := txs[int(index)]
	meta := &chain.TxMeta{BlockID: header.ID(), Index: uint64(index)}
	baseGasPrice := api.getBaseGasPrice(header)
	return meterTxToEthTx(t, meta, header, api.chainID, baseGasPrice, nil), nil
}

func (api *EthAPI) GetBlockTransactionCountByNumber(blockNr string) (string, error) {
	header, err := api.resolveBlockNumber(blockNr)
	if err != nil {
		return "0x0", nil
	}
	blk, err := api.chain.GetBlock(header.ID())
	if err != nil {
		return "0x0", nil
	}
	return hexUint64(uint64(len(blk.Txs))), nil
}

func (api *EthAPI) Call(args CallArgs, blockNrOrHash string) (string, error) {
	header, err := api.resolveBlockNumber(blockNrOrHash)
	if err != nil {
		return "0x", err
	}
	output, err := api.execCall(args, header)
	if err != nil {
		return "0x", err
	}
	return hexutil.Encode(output.Data), nil
}

func (api *EthAPI) EstimateGas(args CallArgs, blockNrOrHash *string) (string, error) {
	blockNr := "latest"
	if blockNrOrHash != nil {
		blockNr = *blockNrOrHash
	}
	header, err := api.resolveBlockNumber(blockNr)
	if err != nil {
		return "0x0", err
	}
	output, err := api.execCall(args, header)
	if err != nil {
		return "0x0", err
	}

	gasUsed := args.gasLimit() - output.LeftOverGas
	// Add intrinsic gas
	data := args.dataBytes()
	intrinsic := uint64(21000)
	for _, b := range data {
		if b == 0 {
			intrinsic += 4
		} else {
			intrinsic += 16
		}
	}
	total := gasUsed + intrinsic
	// Add 30% buffer
	total = total * 13 / 10
	return hexUint64(total), nil
}

func (api *EthAPI) SendRawTransaction(rawTx string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(rawTx, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid raw tx: %v", err)
	}
	ethTx := types.Transaction{}
	if err := ethTx.UnmarshalBinary(raw); err != nil {
		return "", fmt.Errorf("decode tx: %v", err)
	}
	best := api.chain.BestBlock()
	genID, _ := api.chain.GetAncestorBlockID(best.Header().ID(), 0)
	chainTag := genID[len(genID)-1]
	blockRef := tx.NewBlockRefFromID(best.ID())
	nativeTx, err := tx.NewTransactionFromEthTx(&ethTx, chainTag, blockRef, true)
	if err != nil {
		return "", fmt.Errorf("convert tx: %v", err)
	}
	if err := api.txPool.Add(nativeTx); err != nil {
		api.logger.Warn("failed to add tx to pool", "err", err)
		return "", err
	}
	return nativeTx.ID().String(), nil
}

func (api *EthAPI) GetLogs(filter LogFilterArgs) ([]interface{}, error) {
	fromBlock := uint32(0)
	toBlock := api.chain.BestBlock().Number()

	if filter.BlockHash != nil {
		header, err := api.chain.GetBlockHeader(meter.Bytes32(*filter.BlockHash))
		if err != nil {
			return nil, nil
		}
		fromBlock = header.Number()
		toBlock = header.Number()
	} else {
		if filter.FromBlock != nil {
			if n, err := api.blockNumberToUint32(*filter.FromBlock); err == nil {
				fromBlock = n
			}
		}
		if filter.ToBlock != nil {
			if n, err := api.blockNumberToUint32(*filter.ToBlock); err == nil {
				toBlock = n
			}
		}
	}

	return api.queryLogs(fromBlock, toBlock, filter.Addresses, filter.Topics)
}

func (api *EthAPI) FeeHistory(blockCount hexutil.Uint64, newestBlock string, rewardPercentiles []float64) (map[string]interface{}, error) {
	header, err := api.resolveBlockNumber(newestBlock)
	if err != nil {
		return nil, err
	}
	count := uint64(blockCount)
	if count > 1024 {
		count = 1024
	}
	endNum := uint64(header.Number())
	startNum := uint64(0)
	if endNum >= count {
		startNum = endNum - count + 1
	}

	baseFees := make([]string, 0, count+1)
	gasUsedRatios := make([]float64, 0, count)

	for i := startNum; i <= endNum; i++ {
		h, err := api.chain.GetTrunkBlockHeader(uint32(i))
		if err != nil {
			baseFees = append(baseFees, "0x0")
			gasUsedRatios = append(gasUsedRatios, 0)
			continue
		}
		s, err := api.stateCreator.NewState(h.StateRoot())
		if err != nil {
			baseFees = append(baseFees, "0x0")
			gasUsedRatios = append(gasUsedRatios, 0)
			continue
		}
		baseGas := builtin.Params.Native(s).Get(meter.KeyBaseGasPrice)
		baseFees = append(baseFees, hexBig(baseGas))
		gasLimit := h.GasLimit()
		gasUsed := h.GasUsed()
		ratio := float64(0)
		if gasLimit > 0 {
			ratio = float64(gasUsed) / float64(gasLimit)
		}
		gasUsedRatios = append(gasUsedRatios, ratio)
	}
	// One extra baseFee for the next block
	baseFees = append(baseFees, baseFees[len(baseFees)-1])

	return map[string]interface{}{
		"oldestBlock":  hexUint64(startNum),
		"baseFeePerGas": baseFees,
		"gasUsedRatio":  gasUsedRatios,
	}, nil
}

func (api *EthAPI) GetBlockReceipts(blockNr string) ([]interface{}, error) {
	header, err := api.resolveBlockNumber(blockNr)
	if err != nil {
		return nil, nil
	}
	blk, err := api.chain.GetBlock(header.ID())
	if err != nil {
		return nil, nil
	}
	receipts, err := api.chain.GetBlockReceipts(header.ID())
	if err != nil {
		return nil, err
	}
	baseGasPrice := api.getBaseGasPrice(header)
	result := make([]interface{}, 0, len(blk.Txs))
	cumGasUsed := uint64(0)
	for i, t := range blk.Txs {
		if i >= len(receipts) {
			break
		}
		cumGasUsed += receipts[i].GasUsed
		meta := &chain.TxMeta{BlockID: header.ID(), Index: uint64(i)}
		result = append(result, meterReceiptToEthReceipt(receipts[i], t, meta, header, api.chainID, baseGasPrice, cumGasUsed))
	}
	return result, nil
}

// ---------- WebSocket Subscriptions ----------

// NewHeads sends a notification each time a new block is appended to the chain.
// Mapped to eth_subscribe("newHeads") by the geth rpc.Server.
func (api *EthAPI) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}
	sub := notifier.CreateSubscription()
	go func() {
		ticker := api.chain.NewTicker()
		for {
			select {
			case <-sub.Err():
				return
			case <-ticker.C():
				best := api.chain.BestBlock()
				blk, err := api.buildEthBlock(best, false)
				if err != nil {
					return
				}
				notifier.Notify(sub.ID, blk) //nolint:errcheck
			}
		}
	}()
	return sub, nil
}

// Logs sends a notification for each log matching the filter.
// Mapped to eth_subscribe("logs", filter) by the geth rpc.Server.
func (api *EthAPI) Logs(ctx context.Context, filter LogSubFilter) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}
	sub := notifier.CreateSubscription()
	go func() {
		best := api.chain.BestBlock()
		blockReader := api.chain.NewBlockReader(best.ID())
		ticker := api.chain.NewTicker()
		for {
			blocks, err := blockReader.Read()
			if err != nil {
				return
			}
			for _, blk := range blocks {
				header := blk.Header()
				receipts, err := api.chain.GetBlockReceipts(blk.ID())
				if err != nil {
					continue
				}
				logIndex := 0
				for i, receipt := range receipts {
					if i >= len(blk.Txs) {
						break
					}
					t := blk.Txs[i]
					meta := &chain.TxMeta{BlockID: header.ID(), Index: uint64(i)}
					for _, output := range receipt.Outputs {
						for _, event := range output.Events {
							if matchesLogFilter(event, filter) {
								notifier.Notify(sub.ID, meterLogToEthLog(event, header, t, meta, logIndex)) //nolint:errcheck
							}
							logIndex++
						}
					}
				}
			}
			if len(blocks) == 0 {
				select {
				case <-sub.Err():
					return
				case <-ticker.C():
				}
			} else {
				select {
				case <-sub.Err():
					return
				default:
				}
			}
		}
	}()
	return sub, nil
}

// ---------- Net / Web3 / RPC / EVM ----------

type NetAPI struct {
	chainID *big.Int
}

func (api *NetAPI) Version() string {
	return api.chainID.String()
}

func (api *NetAPI) Listening() bool {
	return false
}

func (api *NetAPI) PeerCount() string {
	return "0x0"
}

type Web3API struct{}

func (api *Web3API) ClientVersion() string {
	return "meter-pov/v1.0"
}

type RPCAPI struct{}

func (api *RPCAPI) Modules() map[string]string {
	return map[string]string{
		"eth":  "1.0",
		"net":  "1.0",
		"web3": "1.0",
	}
}

type EVMAPI struct{}

func (api *EVMAPI) Snapshot() string {
	return "0x0"
}

func (api *EVMAPI) Revert(id string) bool {
	return true
}

// ---------- Debug / Trace ----------

// DebugAPI implements debug_* methods. Tracing requires a native tracer
// implementation; for now these return stubs so clients don't hard-error.
type DebugAPI struct{}

func (api *DebugAPI) TraceTransaction(hash common.Hash, params interface{}) (interface{}, error) {
	return map[string]interface{}{
		"gas":         "0x0",
		"returnValue": "",
		"structLogs":  []interface{}{},
	}, nil
}

func (api *DebugAPI) StorageRangeAt(blkHash common.Hash, txIndex int, addr common.Address, keyStart string, maxResult int) (interface{}, error) {
	return map[string]interface{}{
		"storage": map[string]interface{}{},
		"nextKey": nil,
	}, nil
}

// TraceAPI implements trace_* methods (Parity-style tracing).
// These proxy calls are stubs until native tracing is implemented.
type TraceAPI struct{}

func (api *TraceAPI) Filter(filter interface{}) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (api *TraceAPI) Transaction(hash common.Hash) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (api *TraceAPI) Block(blockNr string) ([]interface{}, error) {
	return []interface{}{}, nil
}

// ---------- Helpers ----------

type CallArgs struct {
	From     *common.Address `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     *hexutil.Bytes  `json:"data"`
	Input    *hexutil.Bytes  `json:"input"`
}

func (args CallArgs) gasLimit() uint64 {
	if args.Gas != nil {
		return uint64(*args.Gas)
	}
	return 0
}

func (args CallArgs) dataBytes() []byte {
	if args.Input != nil {
		return []byte(*args.Input)
	}
	if args.Data != nil {
		return []byte(*args.Data)
	}
	return nil
}

// TopicFilter handles the Ethereum topics encoding where each position can be
// null (match any), a single hash, or an array of hashes (OR match).
type TopicFilter [][]common.Hash

func (t *TopicFilter) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	result := make([][]common.Hash, len(raw))
	for i, r := range raw {
		if string(r) == "null" {
			result[i] = nil
			continue
		}
		var single common.Hash
		if err := json.Unmarshal(r, &single); err == nil {
			result[i] = []common.Hash{single}
			continue
		}
		var arr []common.Hash
		if err := json.Unmarshal(r, &arr); err != nil {
			return err
		}
		result[i] = arr
	}
	*t = TopicFilter(result)
	return nil
}

// AddressOrArray handles a JSON field that can be a single address or array of addresses.
type AddressOrArray []common.Address

func (a *AddressOrArray) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var single common.Address
	if err := json.Unmarshal(data, &single); err == nil {
		*a = AddressOrArray{single}
		return nil
	}
	var arr []common.Address
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*a = AddressOrArray(arr)
	return nil
}

type LogFilterArgs struct {
	FromBlock *string        `json:"fromBlock"`
	ToBlock   *string        `json:"toBlock"`
	Addresses AddressOrArray `json:"address"`
	Topics    TopicFilter    `json:"topics"`
	BlockHash *common.Hash   `json:"blockHash"`
}

// LogSubFilter is used for eth_subscribe("logs", filter).
type LogSubFilter struct {
	Addresses AddressOrArray `json:"address"`
	Topics    TopicFilter    `json:"topics"`
}

func (api *EthAPI) execCall(args CallArgs, header *block.Header) (*runtime.Output, error) {
	s, err := api.stateCreator.NewState(header.StateRoot())
	if err != nil {
		return nil, err
	}
	signer, _ := header.Signer()
	rt := runtime.New(api.chain.NewSeeker(header.ParentID()), s,
		&xenv.BlockContext{
			Beneficiary: header.Beneficiary(),
			Signer:      signer,
			Number:      header.Number(),
			Time:        header.Timestamp(),
			GasLimit:    header.GasLimit(),
			TotalScore:  header.TotalScore(),
		})

	gas := api.callGasLimit
	if args.Gas != nil && uint64(*args.Gas) > 0 {
		gas = uint64(*args.Gas)
	}
	if gas > api.callGasLimit {
		gas = api.callGasLimit
	}

	var to *meter.Address
	if args.To != nil {
		a := meter.Address(*args.To)
		to = &a
	}
	var value *big.Int
	if args.Value != nil {
		value = args.Value.ToInt()
	} else {
		value = new(big.Int)
	}
	var gasPrice *big.Int
	if args.GasPrice != nil {
		gasPrice = args.GasPrice.ToInt()
	} else {
		gasPrice = new(big.Int)
	}

	data := args.dataBytes()
	clause := tx.NewClause(to).WithValue(value).WithData(data)

	origin := meter.Address{}
	if args.From != nil {
		origin = meter.Address(*args.From)
	}
	best := api.chain.BestBlock()
	blockRef := tx.NewBlockRefFromID(best.ID())

	exec, _ := rt.PrepareClause(clause, 0, gas, &xenv.TransactionContext{
		Origin:     origin,
		GasPrice:   gasPrice,
		BlockRef:   blockRef,
		Nonce:      rand.Uint64(),
		ProvedWork: new(big.Int),
	})

	out, _ := exec()
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (api *EthAPI) resolveBlockNumber(blockNr string) (*block.Header, error) {
	switch blockNr {
	case "", "latest", "pending", "safe", "finalized":
		return api.chain.BestBlock().Header(), nil
	case "earliest":
		return api.chain.GenesisBlock().Header(), nil
	default:
		n, err := hexutil.DecodeUint64(blockNr)
		if err != nil {
			// Try as block hash
			blockID, err2 := meter.ParseBytes32(blockNr)
			if err2 != nil {
				return nil, fmt.Errorf("invalid block number: %v", err)
			}
			return api.chain.GetBlockHeader(blockID)
		}
		return api.chain.GetTrunkBlockHeader(uint32(n))
	}
}

func (api *EthAPI) blockNumberToUint32(blockNr string) (uint32, error) {
	switch blockNr {
	case "", "latest", "pending", "safe", "finalized":
		return api.chain.BestBlock().Number(), nil
	case "earliest":
		return 0, nil
	default:
		n, err := hexutil.DecodeUint64(blockNr)
		if err != nil {
			return 0, err
		}
		return uint32(n), nil
	}
}

func (api *EthAPI) getBaseGasPrice(header *block.Header) *big.Int {
	s, err := api.stateCreator.NewState(header.StateRoot())
	if err != nil {
		return new(big.Int)
	}
	return builtin.Params.Native(s).Get(meter.KeyBaseGasPrice)
}

func (api *EthAPI) buildEthBlock(blk *block.Block, fullTx bool) (map[string]interface{}, error) {
	header := blk.Header()
	var receipts tx.Receipts
	if blk.ID().String() != api.chain.GenesisBlock().ID().String() {
		var err error
		receipts, err = api.chain.GetBlockReceipts(blk.ID())
		if err != nil {
			receipts = make([]*tx.Receipt, 0)
		}
	}
	baseGasPrice := api.getBaseGasPrice(header)
	return meterBlockToEthBlock(blk, receipts, fullTx, api.chainID, baseGasPrice), nil
}

func (api *EthAPI) buildPendingTx(t *tx.Transaction) map[string]interface{} {
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
	baseGasPrice := api.getBaseGasPrice(api.chain.BestBlock().Header())
	gasPrice := t.GasPrice(baseGasPrice)
	return map[string]interface{}{
		"hash":             t.ID().String(),
		"blockHash":        nil,
		"blockNumber":      nil,
		"transactionIndex": nil,
		"from":             origin.String(),
		"to":               to,
		"value":            value,
		"input":            input,
		"gas":              hexUint64(t.Gas()),
		"gasPrice":         hexBig(gasPrice),
		"nonce":            hexUint64(t.Nonce()),
		"type":             "0x0",
		"chainId":          hexBig(api.chainID),
	}
}

func (api *EthAPI) queryLogs(fromBlock, toBlock uint32, addresses []common.Address, topics [][]common.Hash) ([]interface{}, error) {
	criteriaSet := buildCriteriaSet(addresses, topics)

	filter := &logdb.EventFilter{
		CriteriaSet: criteriaSet,
		Range: &logdb.Range{
			Unit: logdb.Block,
			From: uint64(fromBlock),
			To:   uint64(toBlock),
		},
		Order: logdb.ASC,
	}

	events, err := api.logDB.FilterEvents(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0, len(events))
	for _, ev := range events {
		evTopics := make([]string, 0)
		for _, t := range ev.Topics {
			if t != nil {
				evTopics = append(evTopics, t.String())
			}
		}
		result = append(result, map[string]interface{}{
			"address":          ev.Address.String(),
			"topics":           evTopics,
			"data":             hexutil.Encode(ev.Data),
			"blockHash":        ev.BlockID.String(),
			"blockNumber":      hexUint64(uint64(ev.BlockNumber)),
			"transactionHash":  ev.TxID.String(),
			"transactionIndex": hexUint64(uint64(ev.Index)),
			"logIndex":         hexUint64(uint64(ev.Index)),
			"removed":          false,
		})
	}
	return result, nil
}

// buildCriteriaSet builds a logdb criteria set from addresses and topics,
// correctly expanding OR-topic positions into multiple criteria.
func buildCriteriaSet(addresses []common.Address, topics [][]common.Hash) []*logdb.EventCriteria {
	// Start with one criteria per address (or one with no address filter).
	bases := make([]*logdb.EventCriteria, 0)
	if len(addresses) == 0 {
		bases = append(bases, &logdb.EventCriteria{})
	} else {
		for _, addr := range addresses {
			a := meter.Address(addr)
			bases = append(bases, &logdb.EventCriteria{Address: &a})
		}
	}

	// For each topic position, expand OR values into multiple criteria.
	for i, topicList := range topics {
		if i >= 5 || len(topicList) == 0 {
			continue // nil/empty means match any — leave criteria.Topics[i] as nil
		}
		expanded := make([]*logdb.EventCriteria, 0, len(bases)*len(topicList))
		for _, base := range bases {
			for _, topic := range topicList {
				c := *base // shallow copy — safe since we only set new pointers
				t := meter.Bytes32(topic)
				c.Topics[i] = &t
				expanded = append(expanded, &c)
			}
		}
		bases = expanded
	}
	return bases
}

// matchesLogFilter reports whether an event matches a log subscription filter.
func matchesLogFilter(event *tx.Event, filter LogSubFilter) bool {
	if len(filter.Addresses) > 0 {
		found := false
		for _, addr := range filter.Addresses {
			if meter.Address(addr) == event.Address {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for i, topicList := range filter.Topics {
		if len(topicList) == 0 {
			continue // null = match any
		}
		if i >= len(event.Topics) {
			return false
		}
		found := false
		for _, topic := range topicList {
			if meter.Bytes32(topic) == event.Topics[i] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
