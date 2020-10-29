// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package debug

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/consensus"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/runtime"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tracers"
	"github.com/dfinlab/meter/trie"
	"github.com/dfinlab/meter/types"
	"github.com/dfinlab/meter/vm"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gorilla/mux"
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

func (d *Debug) handleTxEnv(ctx context.Context, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64) (*runtime.Runtime, *runtime.TransactionExecutor, error) {
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

	// XXX TODO: make sure this won't change anything
	// The reason why we have these lines is interface change of NewConsensusReactor( with private and public key added)
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, nil, utils.Forbidden(errors.New("can not generate private/public key"))
	}

	blsCommon := consensus.NewBlsCommon()

	rt, err := consensus.NewConsensusReactor(nil, d.chain, d.stateC, privKey, &privKey.PublicKey, Magic, blsCommon, make([]*types.Delegate /* FIXME: this is an empty input */, 0)).NewRuntimeForReplay(block.Header())
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

//trace an existed transaction
func (d *Debug) traceTransaction(ctx context.Context, tracer vm.Tracer, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64) (interface{}, error) {
	rt, txExec, err := d.handleTxEnv(ctx, blockID, txIndex, clauseIndex)
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
	if err != nil {
		return err
	}
	res, err := d.traceTransaction(req.Context(), tracer, blockID, txIndex, clauseIndex)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, res)
}

func (d *Debug) debugStorage(ctx context.Context, contractAddress meter.Address, blockID meter.Bytes32, txIndex uint64, clauseIndex uint64, keyStart []byte, maxResult int) (*StorageRangeResult, error) {
	rt, _, err := d.handleTxEnv(ctx, blockID, txIndex, clauseIndex)
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

func (d *Debug) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("/tracers").Methods(http.MethodPost).HandlerFunc(utils.WrapHandlerFunc(d.handleTraceTransaction))
	sub.Path("/storage-range").Methods(http.MethodPost).HandlerFunc(utils.WrapHandlerFunc(d.handleDebugStorage))

}
