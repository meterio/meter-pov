// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type Blocks struct {
	chain *chain.Chain
}

func New(chain *chain.Chain) *Blocks {
	return &Blocks{
		chain,
	}
}

func (b *Blocks) handleGetBestQC(w http.ResponseWriter, req *http.Request) error {
	quorumCert := b.chain.BestQCOrCandidate()
	qc, err := convertQC(quorumCert)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, qc)
}

func (b *Blocks) handleGetBlock(w http.ResponseWriter, req *http.Request) error {
	revision, err := b.parseRevision(mux.Vars(req)["revision"])
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "revision"))
	}
	expanded := req.URL.Query().Get("expanded")
	if expanded != "" && expanded != "false" && expanded != "true" {
		return utils.BadRequest(errors.WithMessage(errors.New("should be boolean"), "expanded"))
	}

	block, err := b.getBlock(revision)
	if err != nil {
		if b.chain.IsNotFound(err) {
			return utils.WriteJSON(w, nil)
		}
		return err
	}
	isTrunk, err := b.isTrunk(block.Header().ID(), block.Header().Number())
	if err != nil {
		return err
	}

	jSummary := buildJSONBlockSummary(block, isTrunk)
	if expanded == "true" {
		var receipts tx.Receipts
		var err error
		var txs tx.Transactions
		if block.Header().ID().String() == b.chain.GenesisBlock().Header().ID().String() {
			// if is genesis

		} else {
			txs = block.Txs
			receipts, err = b.chain.GetBlockReceipts(block.Header().ID())
			if err != nil {
				return err
			}
		}

		return utils.WriteJSON(w, &JSONExpandedBlock{
			jSummary,
			buildJSONEmbeddedTxs(txs, receipts),
		})
	}
	txIds := make([]meter.Bytes32, 0)
	for _, tx := range block.Txs {
		txIds = append(txIds, tx.ID())
	}
	return utils.WriteJSON(w, &JSONCollapsedBlock{jSummary, txIds})
}

func (b *Blocks) parseRevision(revision string) (interface{}, error) {
	if revision == "" || revision == "best" {
		return nil, nil
	}
	if revision == "leaf" {
		return "", nil
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

func (b *Blocks) parseEpoch(epoch string) (uint32, error) {
	n, err := strconv.ParseUint(epoch, 0, 0)
	if err != nil {
		return 0, err
	}
	if n > math.MaxUint32 {
		return 0, errors.New("block number out of max uint32")
	}
	return uint32(n), err
}

func (b *Blocks) getBlock(revision interface{}) (*block.Block, error) {
	switch revision.(type) {
	case meter.Bytes32:
		return b.chain.GetBlock(revision.(meter.Bytes32))
	case uint32:
		return b.chain.GetTrunkBlock(revision.(uint32))
	case string:
		return b.chain.LeafBlock(), nil
	default:
		return b.chain.BestBlock(), nil
	}
}

func (b *Blocks) getKBlockByEpoch(epoch uint64) (*block.Block, error) {
	best := b.chain.BestBlock()
	curEpoch := best.GetBlockEpoch()
	if epoch > curEpoch {
		fmt.Println("requested epoch is too new", "epoch:", epoch)
		return nil, errors.New("requested epoch is too new")
	}

	//fmt.Println("getKBlockByEpoch", "epoch", epoch, "curEpoch", curEpoch)
	delta := uint64(best.Header().Number()) / curEpoch

	var blk *block.Block
	var ht, ep uint64
	var err error
	if curEpoch-epoch <= 5 {
		blk = best
		ht = uint64(best.Header().Number())
		ep = curEpoch
	} else {
		ht := delta * (epoch + 4)
		blk, err = b.chain.GetTrunkBlock(uint32(ht))
		if err != nil {
			fmt.Println("get the kblock failed", "epoch", epoch, "error", err)
			return nil, err
		}

		ep = blk.GetBlockEpoch()
		for ep < epoch+1 {
			ht = ht + (4 * delta)
			if ht >= uint64(best.Header().Number()) {
				ht = uint64(best.Header().Number())
				blk = best
				ep = curEpoch
				break
			}

			blk, err = b.chain.GetTrunkBlock(uint32(ht) + uint32(5*delta))
			if err != nil {
				fmt.Println("get the kblock failed", "epoch", epoch, "error", err)
				return nil, err
			}
			ep = blk.GetBlockEpoch()
			//fmt.Println("... height:", blk.Header().Number(), "epoch", ep)
		}
	}

	// find out the close enough search point
	// fmt.Println("start to search kblock", "height:", blk.Header().Number(), "epoch", ep, "target epoch", epoch)
	for ep > epoch {
		blk, err = b.chain.GetTrunkBlock(blk.Header().LastKBlockHeight())
		if err != nil {
			fmt.Println("get the TrunkBlock failed", "epoch", epoch, "error", err)
			return nil, err
		}
		ep = blk.GetBlockEpoch()
		//fmt.Println("...searching height:", blk.Header().Number(), "epoch", ep)
	}

	//fmt.Println("get the kblock", "height:", blk.Header().Number(), "epoch", ep)
	if ep == epoch {
		return blk, nil
	}

	// now ep < epoch for some reason.
	ht = uint64(blk.Header().Number() + 1)
	count := 0
	for {
		blk, err = b.chain.GetTrunkBlock(uint32(ht))
		if err != nil {
			fmt.Println("get the TrunkBlock failed", "epoch", epoch, "error", err)
			return nil, err
		}
		ep = blk.GetBlockEpoch()

		if ep == epoch && blk.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
			return blk, nil // find it !!!
		}
		if ep > epoch {
			return nil, errors.New("can not find the kblock")
		}

		count++
		if count >= 2000 {
			return nil, errors.New("can not find the kblock")
		}
		//fmt.Println("...final search. height:", ht, "epoch", ep)
	}
}

func (b *Blocks) isTrunk(blkID meter.Bytes32, blkNum uint32) (bool, error) {
	best := b.chain.BestBlock()
	ancestorID, err := b.chain.GetAncestorBlockID(best.Header().ID(), blkNum)
	if err != nil {
		return false, err
	}
	return ancestorID == blkID, nil
}

func (b *Blocks) handleGetQC(w http.ResponseWriter, req *http.Request) error {
	revision, err := b.parseRevision(mux.Vars(req)["revision"])
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "revision"))
	}
	block, err := b.getBlock(revision)
	if err != nil {
		if b.chain.IsNotFound(err) {
			return utils.WriteJSON(w, nil)
		}
		return err
	}
	_, err = b.isTrunk(block.Header().ID(), block.Header().Number())
	if err != nil {
		return err
	}
	qc, err := convertQC(block.QC)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, qc)
}

func (b *Blocks) handleGetEpochPowInfo(w http.ResponseWriter, req *http.Request) error {
	epoch, err := b.parseEpoch(mux.Vars(req)["epoch"])
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "epoch"))
	}

	block, err := b.getKBlockByEpoch(uint64(epoch))
	if err != nil {
		if b.chain.IsNotFound(err) {
			return utils.WriteJSON(w, nil)
		}
		return utils.BadRequest(errors.WithMessage(err, "can't locate kblock within epoch"))
	}

	jEpoch := buildJSONEpoch(block)
	if jEpoch == nil {
		return utils.BadRequest(errors.WithMessage(errors.New("json marshal"), "can't marshal json object for epoch"))
	}

	return utils.WriteJSON(w, jEpoch)
}

func (b *Blocks) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/qc/{revision}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(b.handleGetQC))
	sub.Path("/{revision}").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(b.handleGetBlock))
	sub.Path("/epoch/{epoch}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(b.handleGetEpochPowInfo))
}
