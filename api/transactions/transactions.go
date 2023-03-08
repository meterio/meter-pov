// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package transactions

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
	"github.com/pkg/errors"
)

const (
	RecentTxLimit = 10
)

type Transactions struct {
	chain *chain.Chain
	pool  *txpool.TxPool
}

func New(chain *chain.Chain, pool *txpool.TxPool) *Transactions {
	return &Transactions{
		chain,
		pool,
	}
}

func (t *Transactions) getRawTransaction(txID meter.Bytes32, blockID meter.Bytes32, allowPending bool) (*rawTransaction, error) {
	txMeta, err := t.chain.GetTransactionMeta(txID, blockID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			if allowPending {
				if pending := t.pool.Get(txID); pending != nil {
					raw, err := rlp.EncodeToBytes(pending)
					if err != nil {
						return nil, err
					}
					return &rawTransaction{
						RawTx: RawTx{hexutil.Encode(raw)},
					}, nil
				}
			}
			return nil, nil
		}
		return nil, err
	}
	tx, err := t.chain.GetTransaction(txMeta.BlockID, txMeta.Index)
	if err != nil {
		return nil, err
	}
	block, err := t.chain.GetBlock(txMeta.BlockID)
	if err != nil {
		return nil, err
	}
	raw, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	return &rawTransaction{
		RawTx: RawTx{hexutil.Encode(raw)},
		Meta: TxMeta{
			BlockID:        block.ID(),
			BlockNumber:    block.Number(),
			BlockTimestamp: block.Timestamp(),
		},
	}, nil
}

func (t *Transactions) getTransactionByID(txID meter.Bytes32, blockID meter.Bytes32, allowPending bool) (*Transaction, error) {
	txMeta, err := t.chain.GetTransactionMeta(txID, blockID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			if allowPending {
				if pending := t.pool.Get(txID); pending != nil {
					return convertTransaction(pending, nil, 0)
				}
			}
			return nil, nil
		}
		return nil, err
	}
	tx, err := t.chain.GetTransaction(txMeta.BlockID, txMeta.Index)
	if err != nil {
		return nil, err
	}
	h, err := t.chain.GetBlockHeader(txMeta.BlockID)
	if err != nil {
		return nil, err
	}
	return convertTransaction(tx, h, txMeta.Index)
}

// GetTransactionReceiptByID get tx's receipt
func (t *Transactions) getTransactionReceiptByID(txID meter.Bytes32, blockID meter.Bytes32) (*Receipt, error) {
	txMeta, err := t.chain.GetTransactionMeta(txID, blockID)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	tx, err := t.chain.GetTransaction(txMeta.BlockID, txMeta.Index)
	if err != nil {
		return nil, err
	}
	h, err := t.chain.GetBlockHeader(txMeta.BlockID)
	if err != nil {
		return nil, err
	}
	// disallow getting receipt for tx after best block
	bestBlock := t.chain.BestBlock()
	if h.Number() > bestBlock.Number() {
		return nil, nil
	}
	receipt, err := t.chain.GetTransactionReceipt(txMeta.BlockID, txMeta.Index)
	if err != nil {
		return nil, err
	}
	return convertReceipt(receipt, h, tx)
}

func (t *Transactions) handleSendEthRawTransaction(w http.ResponseWriter, req *http.Request) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	if m == nil {
		return utils.BadRequest(errors.New("body: empty body"))
	}

	var sendTx = func(tx *tx.Transaction) error {
		signer, _ := tx.Signer()
		fmt.Println("tx from black listed address, skip adding this to txpool")
		if strings.ToLower(signer.String()) == "0x0e369a2e02912dba872e72d6c0b661e9617e0d9c" {
			return errors.New("blacklisted address, not allowed in txpool")
		}
		if err := t.pool.Add(tx); err != nil {
			fmt.Println("txpool failed to add tx, error: ", err)
			if txpool.IsBadTx(err) {
				return utils.BadRequest(err)
			}
			if txpool.IsTxRejected(err) {
				return utils.Forbidden(err)
			}
			return utils.BadRequest(err)
		}
		return utils.WriteJSON(w, map[string]string{
			"id": tx.ID().String(),
		})
	}

	if hasKey(m, "raw") {
		raw := strings.Replace(m["raw"].(string), "0x", "", 1)
		rawBytes, _ := hex.DecodeString(raw)
		ethTx := types.Transaction{}
		stream := rlp.NewStream(bytes.NewReader(rawBytes), 0)
		err := ethTx.DecodeRLP(stream)
		if err != nil {
			fmt.Println("raw tx ERR: ", err)
		}
		bestBlock := t.chain.BestBlock()
		genID, _ := t.chain.GetAncestorBlockID(bestBlock.BlockHeader.ID(), 0)
		chainTag := genID[len(genID)-1]
		bestBlockID := bestBlock.BlockHeader.ID()
		blockRef := tx.NewBlockRefFromID(bestBlockID)
		nativeTx, err := tx.NewTransactionFromEthTx(&ethTx, chainTag, blockRef, true)
		if err != nil {
			return utils.BadRequest(err)
		} else {
			return sendTx(nativeTx)
		}
	}
	return utils.BadRequest(err)
}

func (t *Transactions) handleSendTransaction(w http.ResponseWriter, req *http.Request) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	if m == nil {
		return utils.BadRequest(errors.New("body: empty body"))
	}
	var sendTx = func(tx *tx.Transaction) error {
		if err := t.pool.Add(tx); err != nil {
			if txpool.IsBadTx(err) {
				return utils.BadRequest(err)
			}
			if txpool.IsTxRejected(err) {
				if tx != nil {
					fmt.Println(err, tx.String())
				}
				return utils.Forbidden(err)
			}
			if tx != nil {
				fmt.Println(err, tx.String())
			}
			return err
		}
		return utils.WriteJSON(w, map[string]string{
			"id": tx.ID().String(),
		})
	}
	reader := bytes.NewReader(data)
	if hasKey(m, "raw") {
		var rawTx *RawTx
		if err := utils.ParseJSON(reader, &rawTx); err != nil {
			return utils.BadRequest(errors.WithMessage(err, "body"))
		}
		tx, err := rawTx.decode()
		if err != nil {
			return utils.BadRequest(errors.WithMessage(err, "raw"))
		}
		return sendTx(tx)
	} else if hasKey(m, "signature") {
		var stx *SignedTx
		if err := utils.ParseJSON(reader, &stx); err != nil {
			return utils.BadRequest(errors.WithMessage(err, "body"))
		}
		tx, err := stx.decode()
		if err != nil {
			return utils.BadRequest(err)
		}
		return sendTx(tx)
	} else {
		var ustx *UnSignedTx
		if err := utils.ParseJSON(reader, &ustx); err != nil {
			return utils.BadRequest(errors.WithMessage(err, "body"))
		}
		tx, err := ustx.decode()
		if err != nil {
			return utils.BadRequest(err)
		}
		return utils.WriteJSON(w, map[string]string{
			"signingHash": tx.SigningHash().String(),
		})
	}
}

func (t *Transactions) handleGetTransactionByID(w http.ResponseWriter, req *http.Request) error {
	id := mux.Vars(req)["id"]
	txID, err := meter.ParseBytes32(id)
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "id"))
	}
	head, err := t.parseHead(req.URL.Query().Get("head"))
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "head"))
	}
	h, err := t.chain.GetBlockHeader(head)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return utils.BadRequest(errors.WithMessage(err, "head"))
		}
		return err
	}
	raw := req.URL.Query().Get("raw")
	if raw != "" && raw != "false" && raw != "true" {
		return utils.BadRequest(errors.WithMessage(errors.New("should be boolean"), "raw"))
	}
	pending := req.URL.Query().Get("pending")
	if pending != "" && pending != "false" && pending != "true" {
		return utils.BadRequest(errors.WithMessage(errors.New("should be boolean"), "pending"))
	}
	if raw == "true" {
		tx, err := t.getRawTransaction(txID, h.ID(), pending == "true")
		if err != nil {
			return err
		}
		return utils.WriteJSON(w, tx)
	}
	tx, err := t.getTransactionByID(txID, h.ID(), pending == "true")
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, tx)

}

func (t *Transactions) handleGetTransactionReceiptByID(w http.ResponseWriter, req *http.Request) error {
	id := mux.Vars(req)["id"]
	txID, err := meter.ParseBytes32(id)
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "id"))
	}
	head, err := t.parseHead(req.URL.Query().Get("head"))
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "head"))
	}
	h, err := t.chain.GetBlockHeader(head)
	if err != nil {
		if t.chain.IsNotFound(err) {
			return utils.BadRequest(errors.WithMessage(err, "head"))
		}
		return err
	}
	receipt, err := t.getTransactionReceiptByID(txID, h.ID())
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, receipt)
}

func (t *Transactions) parseHead(head string) (meter.Bytes32, error) {
	if head == "" {
		return t.chain.BestBlock().ID(), nil
	}
	h, err := meter.ParseBytes32(head)
	if err != nil {
		return meter.Bytes32{}, err
	}
	return h, nil
}

func (t *Transactions) handleGetRecentTransactions(w http.ResponseWriter, req *http.Request) error {
	recentTxs := make([]*Transaction, 0)
	best := t.chain.BestBlock()
	var err error
	for best.Number() > 0 {
		blockHeader := best.Header()
		for _, tx := range best.Txs {
			txMeta, err := t.chain.GetTransactionMeta(tx.ID(), blockHeader.ID())
			if err != nil {
				if t.chain.IsNotFound(err) {
					continue
				}
				continue
			}
			converted, err := convertTransaction(tx, blockHeader, txMeta.Index)
			if err != nil {
				continue
			}
			recentTxs = append(recentTxs, converted)
			if len(recentTxs) == RecentTxLimit {
				break
			}
		}

		if len(recentTxs) == RecentTxLimit {
			break
		}
		parentID := blockHeader.ParentID()
		best, err = t.chain.GetBlock(parentID)
		if err != nil {
			break
		}
	}

	return utils.WriteJSON(w, recentTxs)
}

func (t *Transactions) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(t.handleSendTransaction))
	sub.Path("/eth").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(t.handleSendEthRawTransaction))
	sub.Path("/recent").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(t.handleGetRecentTransactions))
	sub.Path("/{id}").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(t.handleGetTransactionByID))
	sub.Path("/{id}/receipt").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(t.handleGetTransactionReceiptByID))
}
