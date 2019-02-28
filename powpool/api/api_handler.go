// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package api

import (
	"bytes"
	"fmt"
	// "strings"
	"encoding/hex"
	"io/ioutil"
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
	hexBytes, _ := ioutil.ReadAll(req.Body)
	fmt.Println(string(hexBytes))
	actualBytes, _ := hex.DecodeString(string(hexBytes))
	newPowBlock := wire.MsgBlock{}
	err := newPowBlock.Deserialize(bytes.NewReader(actualBytes))
	if err != nil {
		fmt.Println("Could not deserialize pow block", err)
		return err
	}

	fmt.Println("Recved Pow Block: ", newPowBlock)

	info := powpool.NewPowBlockInfoFromPowBlock(&newPowBlock)
	h.powPool.Add(info)

	fmt.Println("Added to pool:", info.ToString())
	return nil
}

func (h *ApiHandler) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(h.handleRecvPowMessage))
}
