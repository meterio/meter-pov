// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/pkg/errors"
)

type AccountLock struct {
	chain        *chain.Chain
	stateCreator *state.Creator
}

func New(chain *chain.Chain,
	stateCreator *state.Creator) *AccountLock {
	return &AccountLock{chain: chain, stateCreator: stateCreator}
}

func (a *AccountLock) handleGetAccountLockProfile(w http.ResponseWriter, req *http.Request) error {
	h, err := a.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	state, err := a.stateCreator.NewState(h.StateRoot())
	if err != nil {
		return err
	}
	list := state.GetProfileList()
	profileList := convertProfileList(list)
	return utils.WriteJSON(w, profileList)
}

func (a *AccountLock) handleRevision(revision string) (*block.Header, error) {
	if revision == "" || revision == "best" {
		return a.chain.BestBlock().Header(), nil
	}
	if len(revision) == 66 || len(revision) == 64 {
		blockID, err := meter.ParseBytes32(revision)
		if err != nil {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		h, err := a.chain.GetBlockHeader(blockID)
		if err != nil {
			if a.chain.IsNotFound(err) {
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
	h, err := a.chain.GetTrunkBlockHeader(uint32(n))
	if err != nil {
		if a.chain.IsNotFound(err) {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		return nil, err
	}
	return h, nil
}

func (a *AccountLock) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/profiles").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(a.handleGetAccountLockProfile))
}
