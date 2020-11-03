// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package slashing

import (
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/script/staking"
	"github.com/gorilla/mux"
)

type Slashing struct {
}

func New() *Slashing {
	return &Slashing{}
}

func (sl *Slashing) handleGetDelegateJailedList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestInJailList()
	if err != nil {
		return err
	}
	jailedList := convertJailedList(list)
	return utils.WriteJSON(w, jailedList)
}

func (sl *Slashing) handleGetDelegateStatsList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestStatisticsList()
	if err != nil {
		return err
	}
	statsList := convertStatisticsList(list)
	return utils.WriteJSON(w, statsList)
}

func (sl *Slashing) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/injail").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(sl.handleGetDelegateJailedList))
	sub.Path("/statistics").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(sl.handleGetDelegateStatsList))

}
