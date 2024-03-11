// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks_test

import (
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/blocks"
	meter_block "github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

const (
	testAddress = "56e81f171bcc55a6ff8345e692c0f86e5b48e01a"
	testPrivHex = "efa321f290811731036e5eccd373114e5186d9fe419081f5a607231279d5ef01"
)

var blk *meter_block.Block
var ts *httptest.Server

var invalidBytes32 = "0x000000000000000000000000000000000000000000000000000000000000000g" //invlaid bytes32
var invalidNumberRevision = "4294967296"                                                  //invalid block number

func TestBlock(t *testing.T) {
	initBlockServer(t)
	defer ts.Close()
	//invalid block id
	res, statusCode := httpGet(t, ts.URL+"/blocks/"+invalidBytes32)
	assert.Equal(t, http.StatusBadRequest, statusCode)
	//invalid block number
	res, statusCode = httpGet(t, ts.URL+"/blocks/"+invalidNumberRevision)
	assert.Equal(t, http.StatusBadRequest, statusCode)

	res, statusCode = httpGet(t, ts.URL+"/blocks/"+blk.ID().String())
	rb := new(blocks.JSONBlockSummary)
	if err := json.Unmarshal(res, &rb); err != nil {
		t.Fatal(err)
	}
	checkBlock(t, blk, rb)
	assert.Equal(t, http.StatusOK, statusCode)

	res, statusCode = httpGet(t, ts.URL+"/blocks/1")
	if err := json.Unmarshal(res, &rb); err != nil {
		t.Fatal(err)
	}
	checkBlock(t, blk, rb)
	assert.Equal(t, http.StatusOK, statusCode)

	res, statusCode = httpGet(t, ts.URL+"/blocks/best")
	if err := json.Unmarshal(res, &rb); err != nil {
		t.Fatal(err)
	}
	checkBlock(t, blk, rb)
	assert.Equal(t, http.StatusOK, statusCode)

}

func initBlockServer(t *testing.T) {
	meter.InitBlockChainConfig("test")
	db, _ := lvldb.NewMem()
	stateC := state.NewCreator(db)
	gene := genesis.NewDevnet()

	b, _, err := gene.Build(stateC)
	if err != nil {
		t.Fatal(err)
	}
	chain, _ := chain.New(db, b, false)
	addr := meter.BytesToAddress([]byte("to"))
	cla := tx.NewClause(&addr).WithValue(big.NewInt(10000))
	tx := new(tx.Builder).
		ChainTag(chain.Tag()).
		GasPriceCoef(1).
		Expiration(10).
		Gas(21000).
		Nonce(1).
		Clause(cla).
		BlockRef(tx.NewBlockRef(0)).
		Build()

	sig, err := crypto.Sign(tx.SigningHash().Bytes(), genesis.DevAccounts()[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	tx = tx.WithSignature(sig)
	packer := packer.New(chain, stateC, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address)
	flow, err := packer.Mock(b.Header(), uint64(time.Now().Unix()), 2000000, &meter.Address{})
	if err != nil {
		t.Fatal(err)
	}
	err = flow.Adopt(tx)
	if err != nil {
		t.Fatal(err)
	}
	block, stage, receipts, err := flow.Pack(genesis.DevAccounts()[0].PrivateKey, meter_block.MBlockType, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Commit(); err != nil {
		t.Fatal(err)
	}
	block.SetQC(&meter_block.QuorumCert{QCHeight: 0, QCRound: 0, EpochID: 0})
	escortQC := &meter_block.QuorumCert{QCHeight: block.Number(), QCRound: 1, EpochID: 0, VoterMsgHash: block.VotingHash()}
	if _, err := chain.AddBlock(block, escortQC, receipts); err != nil {
		t.Fatal(err)
	}
	router := mux.NewRouter()
	blocks.New(chain, stateC).Mount(router, "/blocks")
	ts = httptest.NewServer(router)
	blk = block
}

func checkBlock(t *testing.T, expBl *meter_block.Block, actBl *blocks.JSONBlockSummary) {
	header := expBl.Header()
	assert.Equal(t, header.Number(), actBl.Number, "Number should be equal")
	assert.Equal(t, header.ID(), actBl.ID, "Hash should be equal")
	assert.Equal(t, header.ParentID(), actBl.ParentID, "ParentID should be equal")
	assert.Equal(t, header.Timestamp(), actBl.Timestamp, "Timestamp should be equal")
	assert.Equal(t, header.TotalScore(), actBl.TotalScore, "TotalScore should be equal")
	assert.Equal(t, header.GasLimit(), actBl.GasLimit, "GasLimit should be equal")
	assert.Equal(t, header.GasUsed(), actBl.GasUsed, "GasUsed should be equal")
	assert.Equal(t, header.Beneficiary(), actBl.Beneficiary, "Beneficiary should be equal")
	assert.Equal(t, header.TxsRoot(), actBl.TxsRoot, "TxsRoot should be equal")
	assert.Equal(t, header.StateRoot(), actBl.StateRoot, "StateRoot should be equal")
	assert.Equal(t, header.ReceiptsRoot(), actBl.ReceiptsRoot, "ReceiptsRoot should be equal")

}

func httpGet(t *testing.T, url string) ([]byte, int) {
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	r, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return r, res.StatusCode
}
