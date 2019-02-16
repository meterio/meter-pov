// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/vechain/thor/powpool"
)

//New return api router
func New(powPool *powpool.PowPool) (http.HandlerFunc, func()) {
	router := mux.NewRouter()

	NewApiHandler().
		Mount(router, "/pow")

	return router.ServeHTTP, func() {}
}
