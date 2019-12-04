// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks

import (
	"net/http"
	"strconv"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
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
	quorumCert := b.chain.BestQC()
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
	blk, err := convertBlock(block, isTrunk)
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, blk)
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
func (b *Blocks) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/qc/{revision}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(b.handleGetQC))
	sub.Path("/{revision}").Methods("GET").HandlerFunc(utils.WrapHandlerFunc(b.handleGetBlock))

}
