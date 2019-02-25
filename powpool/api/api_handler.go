// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package api

import (
	// "bytes"
	// "fmt"
	// "strings"
	"net/http"

	"github.com/btcsuite/btcd/wire"
	"github.com/gorilla/mux"
	"github.com/vechain/thor/api/utils"
	"github.com/vechain/thor/powpool"
	// "github.com/pkg/errors"
	// "github.com/btcsuite/btcd/chaincfg/chainhash"
)

type ApiHandler struct {
	powPool *powpool.PowPool
}

type PowMessage struct {
	Raw string `json:"raw"`
}

func NewApiHandler(powPool *powpool.PowPool) *ApiHandler {
	return &ApiHandler{powPool: powPool}
}

func (h *ApiHandler) handleRecvPowMessage(w http.ResponseWriter, req *http.Request) error {
	// var msg PowMessage
	// if err := utils.ParseJSON(req.Body, &msg); err != nil {
	// return utils.BadRequest(errors.WithMessage(err, "body"))
	// }
	// fmt.Println("RAW: ", msg.Raw)
	//
	// prevHash, _ := chainhash.NewHashFromStr("abcdef0123456789")
	// merkleRootHash, _ := chainhash.NewHashFromStr("0123456789abcdef")
	// newBlock := wire.NewMsgBlock(wire.NewBlockHeader(111, prevHash, merkleRootHash, 2222, 3333))
	// var buf bytes.Buffer
	// newBlock.Serialize(&buf)
	// fmt.Println("BUF: ", buf)
	// powBlock := wire.MsgBlock{}
	// powBlock.Deserialize(strings.NewReader(buf.String())) // req.Body)
	// var hash chainhash.Hash
	// hash = powBlock.Header.BlockHash()

	// fmt.Println("POW BLOCK: ", powBlock)
	// fmt.Println("HASH: ", hash)

	newPowBlock := wire.MsgBlock{}
	newPowBlock.Deserialize(req.Body)

	info := powpool.NewPowBlockInfoFromBlock(&newPowBlock)
	h.powPool.Add(info)

	return nil
}

func (h *ApiHandler) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(h.handleRecvPowMessage))
}
