// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script/accountlock"
	"github.com/pkg/errors"
)

type AccountLock struct {
}

func New() *AccountLock {
	return &AccountLock{}
}

func (a *AccountLock) handleGetAccountLockProfile(w http.ResponseWriter, req *http.Request) error {
	list, err := accountlock.GetLatestProfileList()
	if err != nil {
		return err
	}
	profileList := convertProfileList(list)
	return utils.WriteJSON(w, profileList)
}

func (a *AccountLock) handleGetProfileByID(w http.ResponseWriter, req *http.Request) error {
	list, err := accountlock.GetLatestProfileList()
	if err != nil {
		return err
	}
	id := mux.Vars(req)["address"]
	bytes, err := meter.ParseAddress(id)
	if err != nil {
		return utils.BadRequest(errors.WithMessage(err, "address"))
	}
	s := list.Get(bytes)
	profile := convertProfile(s)
	return utils.WriteJSON(w, profile)
}

func (a *AccountLock) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/profiles").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(a.handleGetAccountLockProfile))
}
