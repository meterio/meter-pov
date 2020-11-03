// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package api

import (
	"net/http"

	"github.com/dfinlab/meter/powpool"
	"github.com/gorilla/mux"
	"github.com/inconshreveable/log15"
)

var (
	log = log15.New("pkg", "powpool/api")
)

//New return api router
func New(powPool *powpool.PowPool) (http.HandlerFunc, func()) {
	router := mux.NewRouter()

	NewApiHandler(powPool).
		Mount(router, "/pow")

	return router.ServeHTTP, func() {}
}
