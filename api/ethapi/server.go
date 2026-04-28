// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package ethapi

import (
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/co"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/txpool"
)

func getChainID() *big.Int {
	if meter.IsMainNet() {
		return big.NewInt(meter.MainnetChainID)
	}
	return big.NewInt(meter.TestnetChainID)
}

func StartEthRPC(
	chain *chain.Chain,
	stateCreator *state.Creator,
	txPool *txpool.TxPool,
	logDB *logdb.LogDB,
	callGasLimit uint64,
	addr string,
) (string, func()) {
	chainID := getChainID()
	server := rpc.NewServer()

	ethAPI := NewEthAPI(chain, stateCreator, txPool, logDB, chainID, callGasLimit)
	if err := server.RegisterName("eth", ethAPI); err != nil {
		panic(fmt.Sprintf("register eth namespace: %v", err))
	}
	if err := server.RegisterName("net", &NetAPI{chainID: chainID}); err != nil {
		panic(fmt.Sprintf("register net namespace: %v", err))
	}
	if err := server.RegisterName("web3", &Web3API{}); err != nil {
		panic(fmt.Sprintf("register web3 namespace: %v", err))
	}
	if err := server.RegisterName("rpc", &RPCAPI{}); err != nil {
		panic(fmt.Sprintf("register rpc namespace: %v", err))
	}
	if err := server.RegisterName("evm", &EVMAPI{}); err != nil {
		panic(fmt.Sprintf("register evm namespace: %v", err))
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Sprintf("listen eth-rpc addr [%v]: %v", addr, err))
	}

	handler := server.WebsocketHandler([]string{"*"})
	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	mux.Handle("/", newCORSHandler(server))

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	var goes co.Goes
	goes.Go(func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Warn("eth-rpc server stopped", "err", err)
		}
	})

	slog.Info("ETH JSON-RPC server started", "addr", listener.Addr().String())

	return "http://" + listener.Addr().String() + "/", func() {
		server.Stop()
		if err := srv.Close(); err != nil {
			slog.Warn("could not close eth-rpc server", "err", err)
		}
		goes.Wait()
	}
}

func newCORSHandler(srv *rpc.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		srv.ServeHTTP(w, r)
	})
}
