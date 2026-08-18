package ethapi

import (
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestGetChainID_Testnet(t *testing.T) {
	// Initialize as testnet
	meter.InitBlockChainConfig("test")
	chainID := getChainID()
	assert.Equal(t, big.NewInt(meter.TestnetChainID), chainID)
}

func TestGetChainID_Mainnet(t *testing.T) {
	meter.InitBlockChainConfig("main")
	chainID := getChainID()
	assert.Equal(t, big.NewInt(meter.MainnetChainID), chainID)

	// Reset to test so other tests aren't affected
	meter.InitBlockChainConfig("test")
}

func TestNewCORSHandler_SetsHeaders(t *testing.T) {
	srv := rpc.NewServer()
	handler := newCORSHandler(srv)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "POST, GET, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestNewCORSHandler_OptionsRequest(t *testing.T) {
	srv := rpc.NewServer()
	handler := newCORSHandler(srv)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	// Body should be empty for preflight
	assert.Equal(t, 0, w.Body.Len())
}

func TestNewCORSHandler_GetRequest(t *testing.T) {
	srv := rpc.NewServer()
	handler := newCORSHandler(srv)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// CORS headers should be set even for non-OPTIONS requests
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}
