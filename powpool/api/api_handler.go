// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package api

import (
	// "fmt"
	"github.com/ethereum/go-ethereum/rlp"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/vechain/thor/api/utils"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/powpool"
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

func (ah *ApiHandler) handleRecvPowMessage(w http.ResponseWriter, req *http.Request) error {
	var msg PowMessage
	if err := utils.ParseJSON(req.Body, &msg); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}

	bytes := []byte(msg.Raw)
	powBlockHeader := &block.PowBlockHeader{}
	rlp.DecodeBytes(bytes, powBlockHeader)

	ah.powPool.Add(powBlockHeader)

	return nil
}

func (ah *ApiHandler) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(ah.handleRecvPowMessage))
}
