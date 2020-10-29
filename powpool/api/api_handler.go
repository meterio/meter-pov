// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package api

import (
	"bytes"
	// "strings"
	"encoding/hex"
	"io/ioutil"
	"net/http"

	"github.com/btcsuite/btcd/wire"
	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/powpool"
	"github.com/gorilla/mux"
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
	hexBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error("could not read recved pow message, error:", err)
		return err
	}

	actualBytes, Err := hex.DecodeString(string(hexBytes))
	if Err != nil {
		log.Error("Decode String", "error=", Err)
		return Err
	}

	newPowBlock := wire.MsgBlock{}
	err = newPowBlock.Deserialize(bytes.NewReader(actualBytes))
	if err != nil {
		log.Error("Could not deserialize pow block", "err", err)
		return err
	}

	log.Debug("Recved Pow Block", "hex", string(hexBytes))

	info := powpool.NewPowBlockInfoFromPowBlock(&newPowBlock)
	h.powPool.Add(info)

	return nil
}

func (h *ApiHandler) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(h.handleRecvPowMessage))
}
