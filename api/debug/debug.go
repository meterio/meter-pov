// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package debug

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/runtime"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tracers"
	"github.com/meterio/meter-pov/trie"
	"github.com/meterio/meter-pov/vm"
	"github.com/meterio/meter-pov/xenv"
	"github.com/pkg/errors"
)

type Debug struct {
	chain  *chain.Chain
	stateC *state.Creator
}

var (
	Magic = [4]byte{0x00, 0x00, 0x00, 0x00}
)

func New(chain *chain.Chain, stateC *state.Creator) *Debug {
	return &Debug{
		chain,
		stateC,
	}
}

func (d *Debug) newRuntimeOnBlock(header *block.Header) (*runtime.Runtime, error) {
	signer, err := header.Signer()
	if err != nil {
		return nil, err
	}
	parentHeader, err := d.chain.GetBlockHeader(header.ParentID())
	if err != nil {
		if !d.chain.IsNotFound(err) {
			return nil, err
		}
		return nil, errors.New("parent missing for " + header.String())
	}
	state, err := d.stateC.NewState(parentHeader.StateRoot())
	if err != nil {
		return nil, err
	}

	return runtime.New(
		d.chain.NewSeeker(header.ParentID()),
		state,
		&xenv.BlockContext{
			Beneficiary: header.Beneficiary(),
			Signer:      signer,
			Number:      header.Number(),
			Time:        header.Timestamp(),
			GasLimit:    header.GasLimit(),
			TotalScore:  header.TotalScore(),
		}), nil
}

// copied over from runtime
func ScriptEngineCheck(d []byte) bool {
	return (d[0] == 0xff) && (d[1] == 0xff) && (d[2] == 0xff) && (d[3] == 0xff)
}

func (d *Debug) isScriptEngineClause(blockID meter.Bytes32, txIndex uint64, clauseIndex uint64) bool {
	block, err := d.chain.GetBlock(blockID)
	if err != nil {
		if d.chain.IsNotFound(err) {
			return false
		}
		return false
	}
	txs := block.Transactions()
	if txIndex >= uint64(len(txs)) {
		return false
	}
	if clauseIndex >= uint64(len(txs[txIndex].Clauses())) {
		return false
	}

	tx := txs[txIndex]
	clause := tx.Clauses()[clauseIndex]
	if (clause.Value().Sign() == 0) && (len(clause.Data()) > 16) && ScriptEngineCheck(clause.Data()) {
		return true
	}
	return false
}

func (d *Debug) handleTxEnvNew(ctx context.Context, blockID meter.Bytes32, txIndex uint64) (*runtime.Runtime, *runtime.TransactionExecutor, error) {
	block, err := d.chain.GetBlock(blockID)
	if err != nil {
		if d.chain.IsNotFound(err) {
			return nil, nil, utils.Forbidden(errors.New("block not found"))
		}
		return nil, nil, err
	}
	txs := block.Transactions()
	if txIndex >= uint64(len(txs)) {
		return nil, nil, utils.Forbidden(errors.New("tx index out of range"))
	}

	rt, err := d.newRuntimeOnBlock(block.Header())
	if err != nil {
		return nil, nil, err
	}
	for i, tx := range txs {
		if uint64(i) > txIndex {
			break
		}
		txExec, err := rt.PrepareTransaction(tx)
		if err != nil {
			return nil, nil, err
		}
		if txIndex == uint64(i) {
			return rt, txExec, nil
		}
		for txExec.HasNextClause() {
			if _, _, err := txExec.NextClause(); err != nil {
				return nil, nil, err
			}
		}
		if _, err := txExec.Finalize(); err != nil {
			return nil, nil, err
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
	}
	return nil, nil, utils.Forbidden(errors.New("early reverted"))
}

func (d *Debug) handleClauseEnv(ctx context.Context, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64) (*runtime.Runtime, *runtime.TransactionExecutor, error) {
	block, err := d.chain.GetBlock(blockID)
	if err != nil {
		if d.chain.IsNotFound(err) {
			return nil, nil, utils.Forbidden(errors.New("block not found"))
		}
		return nil, nil, err
	}
	txs := block.Transactions()
	if txIndex >= uint64(len(txs)) {
		return nil, nil, utils.Forbidden(errors.New("tx index out of range"))
	}
	if clauseIndex >= uint64(len(txs[txIndex].Clauses())) {
		return nil, nil, utils.Forbidden(errors.New("clause index out of range"))
	}

	rt, err := d.newRuntimeOnBlock(block.Header())
	if err != nil {
		return nil, nil, err
	}
	for i, tx := range txs {
		if uint64(i) > txIndex {
			break
		}
		txExec, err := rt.PrepareTransaction(tx)
		if err != nil {
			return nil, nil, err
		}
		clauseCounter := uint64(0)
		for txExec.HasNextClause() {
			if txIndex == uint64(i) && clauseIndex == clauseCounter {
				return rt, txExec, nil
			}
			if _, _, err := txExec.NextClause(); err != nil {
				return nil, nil, err
			}
			clauseCounter++
		}
		if _, err := txExec.Finalize(); err != nil {
			return nil, nil, err
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
	}
	return nil, nil, utils.Forbidden(errors.New("early reverted"))
}

// trace an existed transaction
func (d *Debug) traceTransactionWithAllClauses(ctx context.Context, blockID meter.Bytes32, txIndex uint64) (interface{}, error) {
	se := d.isScriptEngineClause(blockID, txIndex, uint64(0))
	if se {
		fmt.Println("Tx has Script Engine clause, skip tracing", se, "blockID", blockID, "txIndex", txIndex, "clauseIndex", 0)
		return nil, nil
	}
	rt, txExec, err := d.handleTxEnvNew(ctx, blockID, txIndex)
	defer func() {
		rt = nil
		txExec = nil
	}()
	if err != nil {
		return nil, err
	}
	code, ok := tracers.CodeByName("callTracer")
	if !ok {
		return nil, errors.New("name: unsupported tracer")
	}
	tracer, err := tracers.New(code)
	if err != nil {
		return nil, errors.New("could not get tracer")
	}
	rt.SetVMConfig(vm.Config{Debug: true, Tracer: tracer})
	clauseIndex := 0

	for txExec.HasNextClause() {
		fmt.Println("Execute ", blockID, txIndex, clauseIndex)
		_, _, err := txExec.NextClause()
		if err != nil {
			return nil, err
		}
		clauseIndex++
	}

	if _, err := txExec.Finalize(); err != nil {
		return nil, err
	}

	return tracer.GetResult()
}

type TraceResult struct {
	Type    string        `json:"type"`
	From    meter.Address `json:"from"`
	To      meter.Address `json:"to"`
	Value   *big.Int      `json:"value"`
	Gas     *big.Int      `json:"gas"`
	GasUsed *big.Int      `json:"gasUsed"`
	Input   string        `json:"input"`
	Output  string        `json:"output"`
	Error   string        `json:"error"`
	Time    string        `json:"time"`
	// Calls
}

// trace an existed transaction

func (d *Debug) execClause(ctx context.Context, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64) (interface{}, error) {
	_, txExec, err := d.handleClauseEnv(ctx, blockID, txIndex, clauseIndex)
	if err != nil {
		fmt.Println("error handle clause env: ", err)
		return nil, err
	}
	gasUsed, output, err := txExec.NextClause()
	if err != nil {
		fmt.Println("error next clause: ", err)
		return nil, err
	}

	errMsg := ""
	if output.VMErr != nil {
		errMsg = output.VMErr.Error()
	}
	b, _ := d.chain.GetBlock(blockID)
	tx := b.Transactions()[txIndex]
	clause := tx.Clauses()[clauseIndex]
	return &TraceResult{
		Type:    "SCRIPTENGINE",
		To:      *clause.To(),
		Input:   "",
		Output:  "0x" + hex.EncodeToString(output.Data),
		Error:   errMsg,
		Value:   clause.Value(),
		Gas:     big.NewInt(int64(tx.Gas())),
		GasUsed: big.NewInt(int64(gasUsed))}, nil
}

// trace an existed transaction
func (d *Debug) traceTransaction(ctx context.Context, tracer vm.Tracer, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64) (interface{}, error) {
	rt, txExec, err := d.handleClauseEnv(ctx, blockID, txIndex, clauseIndex)
	if err != nil {
		return nil, err
	}
	rt.SetVMConfig(vm.Config{Debug: true, Tracer: tracer})
	gasUsed, output, err := txExec.NextClause()
	if err != nil {
		return nil, err
	}
	switch tr := tracer.(type) {
	case *vm.StructLogger:
		return &ExecutionResult{
			Gas:         gasUsed,
			Failed:      output.VMErr != nil,
			ReturnValue: hexutil.Encode(output.Data),
			StructLogs:  formatLogs(tr.StructLogs()),
		}, nil
	case *tracers.Tracer:
		return tr.GetResult()
	default:
		return nil, fmt.Errorf("bad tracer type %T", tracer)
	}
}

type CallTraceResultWithPath struct {
	callTraceResult CallTraceResult
	path            []int
}

func (d *Debug) parseMeterTrace(traceResult []byte, blockHash meter.Bytes32, blockNumber uint64, txHash meter.Bytes32, txIndex uint64) ([]*TraceData, error) {
	var callTraceResult CallTraceResult
	err := json.Unmarshal(traceResult, &callTraceResult)
	if err != nil {
		return make([]*TraceData, 0), err
	}
	return d.convertTraceData(callTraceResult, []uint64{}, blockHash, blockNumber, txHash, txIndex)
}

func (d *Debug) convertTraceData(callTraceResult CallTraceResult, path []uint64, blockHash meter.Bytes32, blockNumber uint64, txHash meter.Bytes32, txIndex uint64) ([]*TraceData, error) {
	value := callTraceResult.Value
	if value == "" {
		value = "0x0"
	}
	gasUsed := callTraceResult.GasUsed
	if gasUsed == "" {
		gasUsed = "0x0"
	}
	gas := callTraceResult.Gas
	if gas == "" {
		gas = "0x0"
	}
	from, err := meter.ParseAddress(callTraceResult.From)
	if err != nil {
		fmt.Println("Error parsing from: ", err, ", set it as default")
		from = meter.Address{}
	}
	to, err := meter.ParseAddress(callTraceResult.To)
	if err != nil {
		fmt.Println("Error parsing to: ", err, ", set it as default")
		to = meter.Address{}
	}
	resultType := strings.ToLower(callTraceResult.Type)
	callType := ""
	txType := ""
	switch resultType {
	case "call":
		fallthrough
	case "callcode":
		fallthrough
	case "delegatecall":
		fallthrough
	case "staticcall":
		callType = resultType
		txType = "call"
	case "create":
		fallthrough
	case "create2":
		callType = "create"
		txType = "create"
	case "selfdestruct":
		callType = "selfdestruct"
		txType = "suicide"
	default:
		callType = resultType
		txType = "unknown"
	}

	datas := []*TraceData{
		&TraceData{
			Action: TraceAction{
				CallType: callType,
				From:     from,
				Input:    callTraceResult.Input,
				Gas:      gas,
				To:       to,
				Value:    value,
			},
			BlockHash:   blockHash,
			BlockNumber: blockNumber,
			Result: TraceDataResult{
				GasUsed: gasUsed,
				Output:  callTraceResult.Output,
			},
			Subtraces:           uint64(len(callTraceResult.Calls)),
			TraceAddress:        path,
			TransactionHash:     txHash,
			TransactionPosition: txIndex,
			Type:                txType,
		},
	}
	for index, call := range callTraceResult.Calls {
		// fmt.Println("convert subcall data: ", call, path, blockHash, blockNumber, txIndex)
		subdatas, err := d.convertTraceData(call, append(path, uint64(index)), blockHash, blockNumber, txHash, txIndex)
		if err != nil {
			fmt.Println("Error happened during subdata query")
		}
		datas = append(datas, subdatas...)
	}
	return datas, nil
}

func (d *Debug) handleRerunTransaction(w http.ResponseWriter, req *http.Request) error {
	clauseIndexStr := mux.Vars(req)["clauseIndex"]
	clauseIndex, err := strconv.Atoi(clauseIndexStr)
	if err != nil {
		return err
	}
	txHashStr := mux.Vars(req)["txhash"]
	txID, err := meter.ParseBytes32(txHashStr)
	if err != nil {
		fmt.Println("error parsing txid", err)
		return err
	}
	best := d.chain.BestBlock()
	txMeta, err := d.chain.GetTransactionMeta(txID, best.ID())
	if err != nil {
		fmt.Println("error getting txmeta", err)
		return err
	}
	fmt.Println("handle trace", "blockID:", txMeta.BlockID, "txIndex:", txMeta.Index, "clauseIndex:", clauseIndex)
	res, err := d.execClause(req.Context(), txMeta.BlockID, txMeta.Index, uint64(clauseIndex))
	if err != nil {
		return err
	}
	fmt.Println("complete handle trace")
	return utils.WriteJSON(w, res)
}

func (d *Debug) handleNewTraceTransaction(w http.ResponseWriter, req *http.Request) error {
	clauseIndexStr := mux.Vars(req)["clauseIndex"]
	clauseIndex, err := strconv.Atoi(clauseIndexStr)
	if err != nil {
		return err
	}
	txHashStr := mux.Vars(req)["txhash"]
	txID, err := meter.ParseBytes32(txHashStr)
	if err != nil {
		fmt.Println("error parsing txid", err)
		return err
	}
	best := d.chain.BestBlock()
	txMeta, err := d.chain.GetTransactionMeta(txID, best.ID())
	if err != nil {
		fmt.Println("error getting txmeta", err)
		return err
	}
	isSE := d.isScriptEngineClause(txMeta.BlockID, txMeta.Index, uint64(clauseIndex))
	var res interface{}
	if isSE {
		fmt.Println("handle trace", "blockID:", txMeta.BlockID, "txIndex:", txMeta.Index, "clauseIndex:", clauseIndex)
		res, err = d.execClause(req.Context(), txMeta.BlockID, txMeta.Index, uint64(clauseIndex))
		if err != nil {
			return err
		}
	} else {
		code, ok := tracers.CodeByName("callTracer")
		if !ok {
			return utils.BadRequest(errors.New("name: unsupported tracer"))
		}
		tracer, err := tracers.New(code)
		if err != nil {
			return err
		}
		res, err = d.traceTransaction(req.Context(), tracer, txMeta.BlockID, txMeta.Index, uint64(clauseIndex))
		if err != nil {
			return err
		}
	}

	fmt.Println("complete handle trace")
	return utils.WriteJSON(w, res)
}

func (d *Debug) handleTraceTransaction(w http.ResponseWriter, req *http.Request) error {
	var opt *TracerOption
	if err := utils.ParseJSON(req.Body, &opt); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	if opt == nil {
		return utils.BadRequest(errors.New("body: empty body"))
	}
	var tracer vm.Tracer
	if opt.Name == "" {
		tracer = vm.NewStructLogger(nil)
	} else {
		name := opt.Name
		if !strings.HasSuffix(name, "Tracer") {
			name += "Tracer"
		}
		code, ok := tracers.CodeByName(name)
		if !ok {
			return utils.BadRequest(errors.New("name: unsupported tracer"))
		}
		tr, err := tracers.New(code)
		if err != nil {
			return err
		}
		tracer = tr
	}
	blockID, txIndex, clauseIndex, err := d.parseTarget(opt.Target)
	fmt.Println("handle tracer", "blockID:", blockID, "txIndex:", txIndex, "clauseIndex:", clauseIndex)
	if err != nil {
		return err
	}
	res, err := d.traceTransaction(req.Context(), tracer, blockID, txIndex, clauseIndex)
	if err != nil {
		return err
	}
	fmt.Println("complete handle tracer")
	return utils.WriteJSON(w, res)
}

func (d *Debug) debugStorage(ctx context.Context, contractAddress meter.Address, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64, keyStart []byte, maxResult int) (*StorageRangeResult, error) {
	rt, _, err := d.handleClauseEnv(ctx, blockID, txIndex, clauseIndex)
	if err != nil {
		return nil, err
	}
	storageTrie, err := rt.State().BuildStorageTrie(contractAddress)
	if err != nil {
		return nil, err
	}
	return storageRangeAt(storageTrie, keyStart, maxResult)
}

func storageRangeAt(t *trie.SecureTrie, start []byte, maxResult int) (*StorageRangeResult, error) {
	it := trie.NewIterator(t.NodeIterator(start))
	result := StorageRangeResult{Storage: StorageMap{}}
	for i := 0; i < maxResult && it.Next(); i++ {
		_, content, _, err := rlp.Split(it.Value)
		if err != nil {
			return nil, err
		}
		v := meter.BytesToBytes32(content)
		e := StorageEntry{Value: &v}
		if preimage := t.GetKey(it.Key); preimage != nil {
			preimage := meter.BytesToBytes32(preimage)
			e.Key = &preimage
		}
		result.Storage[meter.BytesToBytes32(it.Key).String()] = e
	}
	if it.Next() {
		next := meter.BytesToBytes32(it.Key)
		result.NextKey = &next
	}
	return &result, nil
}

func (d *Debug) handleDebugStorage(w http.ResponseWriter, req *http.Request) error {
	var opt *StorageRangeOption
	if err := utils.ParseJSON(req.Body, &opt); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	if opt == nil {
		return utils.BadRequest(errors.New("body: empty body"))
	}
	blockID, txIndex, clauseIndex, err := d.parseTarget(opt.Target)
	if err != nil {
		return err
	}
	var keyStart []byte
	if opt.KeyStart != "" {
		k, err := hexutil.Decode(opt.KeyStart)
		if err != nil {
			return utils.BadRequest(errors.New("keyStart: invalid format"))
		}
		keyStart = k
	}
	res, err := d.debugStorage(req.Context(), opt.Address, blockID, txIndex, clauseIndex, keyStart, opt.MaxResult)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, res)
}

func (d *Debug) parseTarget(target string) (blockID meter.Bytes32, txIndex uint64, clauseIndex uint64, err error) {
	parts := strings.Split(target, "/")
	if len(parts) != 3 {
		return meter.Bytes32{}, 0, 0, utils.BadRequest(errors.New("target:" + target + " unsupported"))
	}
	blockID, err = meter.ParseBytes32(parts[0])
	if err != nil {
		return meter.Bytes32{}, 0, 0, utils.BadRequest(errors.WithMessage(err, "target[0]"))
	}
	if len(parts[1]) == 64 || len(parts[1]) == 66 {
		txID, err := meter.ParseBytes32(parts[1])
		if err != nil {
			return meter.Bytes32{}, 0, 0, utils.BadRequest(errors.WithMessage(err, "target[1]"))
		}
		txMeta, err := d.chain.GetTransactionMeta(txID, blockID)
		if err != nil {
			if d.chain.IsNotFound(err) {
				return meter.Bytes32{}, 0, 0, utils.Forbidden(errors.New("transaction not found"))
			}
			return meter.Bytes32{}, 0, 0, err
		}
		txIndex = txMeta.Index
	} else {
		i, err := strconv.ParseUint(parts[1], 0, 0)
		if err != nil {
			return meter.Bytes32{}, 0, 0, utils.BadRequest(errors.WithMessage(err, "target[1]"))
		}
		txIndex = i
	}
	clauseIndex, err = strconv.ParseUint(parts[2], 0, 0)
	if err != nil {
		return meter.Bytes32{}, 0, 0, utils.BadRequest(errors.WithMessage(err, "target[2]"))
	}
	return
}

func (d *Debug) parseRevision(revision string) (interface{}, error) {
	if revision == "" || revision == "best" {
		return nil, nil
	}
	if len(revision) == 66 || len(revision) == 64 {
		blockID, err := meter.ParseBytes32(revision)
		if err != nil {
			return nil, err
		}
		return blockID, nil
	}
	n, err := strconv.ParseUint(revision, 0, 0)
	if err != nil {
		return nil, err
	}
	if n > math.MaxUint32 {
		return nil, errors.New("block number out of max uint32")
	}
	return uint32(n), err
}

func (d *Debug) getBlock(revision interface{}) (*block.Block, error) {
	switch revision.(type) {
	case meter.Bytes32:
		return d.chain.GetBlock(revision.(meter.Bytes32))
	case uint32:
		return d.chain.GetTrunkBlock(revision.(uint32))
	default:
		return d.chain.BestBlock(), nil
	}
}

func (d *Debug) openEthTraceTransaction(ctx context.Context, txHash meter.Bytes32) ([]*TraceData, error) {
	tx, meta, err := d.chain.GetTrunkTransaction(txHash)
	datas := make([]*TraceData, 0)
	if err != nil {
		fmt.Println("Error happened in GetTrunkTransaction: ", err)
		return datas, err
	}
	_, err = tx.Signer()
	if err != nil {
		fmt.Println("Error: could not get signer:", err)
		return datas, err
	}
	fmt.Println("Openeth trace tx :", tx.ID(), ", BlockID:", meta.BlockID)
	blk, err := d.getBlock(meta.BlockID)
	if err != nil {
		fmt.Println("Could not get block: ", err)
		return datas, err
	}

	res, err := d.traceTransactionWithAllClauses(ctx, blk.ID(), meta.Index)
	if err != nil {
		return datas, err
	}
	if res == nil {
		return datas, nil
	}
	resBytes, ok := res.(json.RawMessage)
	if !ok {
		return datas, errors.New("not expected res")
	}
	clauseDatas, err := d.parseMeterTrace(resBytes, meta.BlockID, uint64(blk.Number()), tx.ID(), meta.Index)
	if err != nil {
		return datas, err
	}
	datas = append(datas, clauseDatas...)
	fmt.Println("Completed openeth trace tx: ", len(datas), "data entries")
	return datas, nil

}

func (d *Debug) handleOpenEthTraceTransaction(w http.ResponseWriter, req *http.Request) error {
	params := make([]meter.Bytes32, 0)

	fmt.Println("handle trace transaction")
	if err := utils.ParseJSON(req.Body, &params); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}

	results := make([]*TraceData, 0)
	for _, txHash := range params {
		txDatas, err := d.openEthTraceTransaction(req.Context(), txHash)
		if err != nil {
			return err
		}
		results = append(results, txDatas...)
	}

	fmt.Println("complete handle trace tx with", len(results), "data entries")
	return utils.WriteJSON(w, results)
}

func (d *Debug) handleOpenEthTraceBlock(w http.ResponseWriter, req *http.Request) error {
	params := make([]string, 0)
	if err := utils.ParseJSON(req.Body, &params); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	if len(params) <= 0 {
		return utils.BadRequest(errors.New("not enough params"))
	}

	revision, err := d.parseRevision(params[0])
	fmt.Println("handle trace block", "revision:", revision)
	if err != nil {
		fmt.Println("Error: could not parse reivision", err)
		return utils.BadRequest(errors.WithMessage(err, "could not parse revision"))
	}
	blk, err := d.getBlock(revision)
	if err != nil {
		fmt.Println("Error: could not get block", err)
		return utils.BadRequest(errors.WithMessage(err, "could not get block"))
	}

	results := make([]*TraceData, 0)
	for _, tx := range blk.Transactions() {
		txDatas, err := d.openEthTraceTransaction(req.Context(), tx.ID())
		if err != nil {
			return utils.BadRequest(err)
		}
		results = append(results, txDatas...)
	}
	fmt.Println("complete handle trace block with", len(results), "data entries")
	return utils.WriteJSON(w, results)
}

func (d *Debug) handleOpenEthTraceFilter(w http.ResponseWriter, req *http.Request) error {
	var opt TraceFilterOptions
	if err := utils.ParseJSON(req.Body, &opt); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	fromBlockNum := uint32(0)
	toBlockNum := uint32(0)

	if opt.FromBlock != "" {
		revision, err := d.parseRevision(opt.FromBlock)
		if err != nil {
			fmt.Println("Error: invalid fromBlock", err)
			return utils.BadRequest(errors.WithMessage(err, "invalid fromBlock"))
		}
		blk, err := d.getBlock(revision)
		if err != nil {
			fmt.Println("Error: could not get block", err)
			return utils.BadRequest(errors.WithMessage(err, "could not get block"))
		}
		fromBlockNum = blk.Number()
	}

	if opt.ToBlock != "" {
		revision, err := d.parseRevision(opt.ToBlock)
		if err != nil {
			fmt.Println("Error: invalid toBlock", err)
			return utils.BadRequest(errors.WithMessage(err, "invalid toBlock"))
		}
		blk, err := d.getBlock(revision)
		if err != nil {
			fmt.Println("Error: could not get block", err)
			return utils.BadRequest(errors.WithMessage(err, "could not get block"))
		}
		toBlockNum = blk.Number()
	}
	fmt.Println("handle trace filter", "fromBlock:", fromBlockNum, ", toBlock:", toBlockNum)

	if fromBlockNum > toBlockNum {
		return utils.BadRequest(errors.New("fromBlock > toBlock"))
	}

	fromAddrMap := make(map[string]bool)
	toAddrMap := make(map[string]bool)
	for _, addr := range opt.FromAddress {
		fromAddrMap[strings.ToLower(addr.String())] = true
	}
	for _, addr := range opt.ToAddress {
		toAddrMap[strings.ToLower(addr.String())] = true
	}

	results := make([]*TraceData, 0)

	// start to filter tx in block range
	num := fromBlockNum
	for num < toBlockNum {
		blk, err := d.getBlock(num)
		if err != nil {
			return utils.BadRequest(errors.WithMessage(err, "could not get block"))
		}
		for _, tx := range blk.Txs {
			txDatas, err := d.openEthTraceTransaction(req.Context(), tx.ID())
			if err != nil {
				return err
			}
			for _, d := range txDatas {
				_, fromExist := fromAddrMap[strings.ToLower(d.Action.From.String())]
				_, toExist := fromAddrMap[strings.ToLower(d.Action.To.String())]

				fromMatch := (len(fromAddrMap) > 0 && fromExist) || len(fromAddrMap) == 0
				toMatch := len(toAddrMap) > 0 && toExist || len(toAddrMap) == 0
				if fromMatch && toMatch {
					results = append(results, d)
				}
			}

		}
		num++
	}

	fmt.Println("complete handle trace filter with ", len(results), "data entries")
	return utils.WriteJSON(w, results)
}

type TokenSupplyDetail struct {
	InitSupply *big.Int `json:"initSupply"`
	TotalAdd   *big.Int `json:"totalAdd"`
	TotalSub   *big.Int `json:"totalSub"`
}

type NativeTokenSupply struct {
	MTR  TokenSupplyDetail `json:"MTR"`
	MTRG TokenSupplyDetail `json:"MTRG"`
}

func (d *Debug) handleSupply(w http.ResponseWriter, req *http.Request) error {
	revision, err := d.parseRevision(mux.Vars(req)["revision"])
	fmt.Println("handle get meter info", "revision:", revision)
	if err != nil {
		fmt.Println("Error: could not parse reivision", err)
		return utils.BadRequest(errors.WithMessage(err, "could not parse revision"))
	}
	blk, err := d.getBlock(revision)
	if err != nil {
		fmt.Println("Error: could not get block", err)
		return utils.BadRequest(errors.WithMessage(err, "could not get block"))
	}

	s, _ := d.stateC.NewState(blk.StateRoot())
	tracker := builtin.MeterTracker.Native(s)
	mtrInitSupply := tracker.GetMeterInitialSupply()
	mtrAddSub := tracker.GetMeterTotalAddSub()
	mtrgInitSupply := tracker.GetMeterGovInitialSupply()
	mtrgAddSub := tracker.GetMeterGovTotalAddSub()

	return utils.WriteJSON(w, NativeTokenSupply{
		MTR:  TokenSupplyDetail{InitSupply: mtrInitSupply, TotalAdd: mtrAddSub.TotalAdd, TotalSub: mtrAddSub.TotalSub},
		MTRG: TokenSupplyDetail{InitSupply: mtrgInitSupply, TotalAdd: mtrgAddSub.TotalAdd, TotalSub: mtrgAddSub.TotalSub},
	})
}

func (d *Debug) handleRevision(revision string) (*block.Header, error) {
	if revision == "" || revision == "best" {
		return d.chain.BestBlock().Header(), nil
	}
	if len(revision) == 66 || len(revision) == 64 {
		blockID, err := meter.ParseBytes32(revision)
		if err != nil {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		h, err := d.chain.GetBlockHeader(blockID)
		if err != nil {
			if d.chain.IsNotFound(err) {
				return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
			}
			return nil, err
		}
		return h, nil
	}
	n, err := strconv.ParseUint(revision, 0, 0)
	if err != nil {
		return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
	}
	if n > math.MaxUint32 {
		return nil, utils.BadRequest(errors.WithMessage(errors.New("block number out of max uint32"), "revision"))
	}
	h, err := d.chain.GetTrunkBlockHeader(uint32(n))
	if err != nil {
		if d.chain.IsNotFound(err) {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		return nil, err
	}
	return h, nil
}

func (d *Debug) handleGetRawStorage(w http.ResponseWriter, req *http.Request) error {
	addr, err := meter.ParseAddress(mux.Vars(req)["address"])
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "address"))
	}
	key, err := meter.ParseBytes32(mux.Vars(req)["key"])
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "key"))
	}
	h, err := d.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	state, err := d.stateC.NewState(h.StateRoot())
	if err != nil {
		return err
	}
	raw := state.GetRawStorage(addr, key)
	if err := state.Err(); err != nil {
		return err
	}
	return utils.WriteJSON(w, map[string]string{"raw": "0x" + hex.EncodeToString(raw)})
}

func (d *Debug) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/tracers").Methods(http.MethodPost).HandlerFunc(utils.WrapHandlerFunc(d.handleTraceTransaction))
	sub.Path("/storage-range").Methods(http.MethodPost).HandlerFunc(utils.WrapHandlerFunc(d.handleDebugStorage))
	sub.Path("/rawstorage/{address}/{key}").Methods(http.MethodGet).HandlerFunc(utils.WrapHandlerFunc(d.handleGetRawStorage))
	sub.Path("/openeth_trace_transaction").Methods(http.MethodPost).HandlerFunc((utils.WrapHandlerFunc(d.handleOpenEthTraceTransaction)))
	sub.Path("/openeth_trace_block").Methods(http.MethodPost).HandlerFunc((utils.WrapHandlerFunc(d.handleOpenEthTraceBlock)))
	sub.Path("/openeth_trace_filter").Methods(http.MethodPost).HandlerFunc((utils.WrapHandlerFunc(d.handleOpenEthTraceFilter)))

	// add api for trace and rerun clause
	sub.Path("/trace/{txhash}/{clauseIndex}").Methods(http.MethodGet).HandlerFunc(utils.WrapHandlerFunc(d.handleNewTraceTransaction))
	sub.Path("/rerun/{txhash}/{clauseIndex}").Methods(http.MethodGet).HandlerFunc(utils.WrapHandlerFunc(d.handleRerunTransaction))

	// add api for supply debug
	sub.Path("/supply").Methods(http.MethodGet).HandlerFunc(utils.WrapHandlerFunc(d.handleSupply))
}
