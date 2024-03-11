// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accounts_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/mux"
	ABI "github.com/meterio/meter-pov/abi"
	"github.com/meterio/meter-pov/api/accounts"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

var sol = `	pragma solidity ^0.4.18;
			contract Test {
    			uint8 value;
    			function add(uint8 a,uint8 b) public pure returns(uint8) {
        			return a+b;
    			}
    			function set(uint8 v) public {
        			value = v;
    			}
			}`

var abiJSON = `[
	{
		"constant": false,
		"inputs": [
			{
				"name": "v",
				"type": "uint8"
			}
		],
		"name": "set",
		"outputs": [],
		"payable": false,
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [
			{
				"name": "a",
				"type": "uint8"
			},
			{
				"name": "b",
				"type": "uint8"
			}
		],
		"name": "add",
		"outputs": [
			{
				"name": "",
				"type": "uint8"
			}
		],
		"payable": false,
		"stateMutability": "pure",
		"type": "function"
	}
]`
var addr = meter.BytesToAddress([]byte("to"))
var value = big.NewInt(10000)
var storageKey = meter.Bytes32{}
var storageValue = byte(1)

var contractAddr meter.Address

var bytecode = common.Hex2Bytes("608060405234801561001057600080fd5b50610125806100206000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806324b8ba5f14604e578063bb4e3f4d14607b575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060cf565b005b348015608657600080fd5b5060b3600480360381019080803560ff169060200190929190803560ff16906020019092919050505060ec565b604051808260ff1660ff16815260200191505060405180910390f35b806000806101000a81548160ff021916908360ff16021790555050565b60008183019050929150505600a165627a7a723058201584add23e31d36c569b468097fe01033525686b59bbb263fb3ab82e9553dae50029")

var runtimeBytecode = common.Hex2Bytes("6080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806324b8ba5f14604e578063bb4e3f4d14607b575b600080fd5b348015605957600080fd5b506079600480360381019080803560ff16906020019092919050505060cf565b005b348015608657600080fd5b5060b3600480360381019080803560ff169060200190929190803560ff16906020019092919050505060ec565b604051808260ff1660ff16815260200191505060405180910390f35b806000806101000a81548160ff021916908360ff16021790555050565b60008183019050929150505600a165627a7a723058201584add23e31d36c569b468097fe01033525686b59bbb263fb3ab82e9553dae50029")

var invalidAddr = "abc"                                                                   //invlaid address
var invalidBytes32 = "0x000000000000000000000000000000000000000000000000000000000000000g" //invlaid bytes32
var invalidNumberRevision = "4294967296"                                                  //invalid block number

var ts *httptest.Server

func TestAccount(t *testing.T) {
	initAccountServer(t)
	defer ts.Close()
	getAccount(t)
	getCode(t)
	getStorage(t)
	deployContractWithCall(t)
	callContract(t)
	batchCall(t)
}

func getAccount(t *testing.T) {
	res, statusCode := httpGet(t, ts.URL+"/accounts/"+invalidAddr)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad address")

	res, statusCode = httpGet(t, ts.URL+"/accounts/"+addr.String()+"?revision="+invalidNumberRevision)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad revision")

	//revision is optional defaut `best`
	res, statusCode = httpGet(t, ts.URL+"/accounts/"+addr.String())
	var acc accounts.Account
	if err := json.Unmarshal(res, &acc); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, math.HexOrDecimal256(*value), acc.Energy, "balance should be equal")
	assert.Equal(t, http.StatusOK, statusCode, "OK")

}

func getCode(t *testing.T) {
	res, statusCode := httpGet(t, ts.URL+"/accounts/"+invalidAddr+"/code")
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad address")

	res, statusCode = httpGet(t, ts.URL+"/accounts/"+contractAddr.String()+"/code?revision="+invalidNumberRevision)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad revision")

	//revision is optional defaut `best`
	res, statusCode = httpGet(t, ts.URL+"/accounts/"+contractAddr.String()+"/code")
	var code map[string]string
	if err := json.Unmarshal(res, &code); err != nil {
		t.Fatal(err)
	}
	c, err := hexutil.Decode(code["code"])
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, runtimeBytecode, c, "code should be equal")
	assert.Equal(t, http.StatusOK, statusCode, "OK")
}

func getStorage(t *testing.T) {
	res, statusCode := httpGet(t, ts.URL+"/accounts/"+invalidAddr+"/storage/"+storageKey.String())
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad address")

	res, statusCode = httpGet(t, ts.URL+"/accounts/"+contractAddr.String()+"/storage/"+invalidBytes32)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad storage key")

	res, statusCode = httpGet(t, ts.URL+"/accounts/"+contractAddr.String()+"/storage/"+storageKey.String()+"?revision="+invalidNumberRevision)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad revision")

	//revision is optional defaut `best`
	res, statusCode = httpGet(t, ts.URL+"/accounts/"+contractAddr.String()+"/storage/"+storageKey.String())
	var value map[string]string
	if err := json.Unmarshal(res, &value); err != nil {
		t.Fatal(err)
	}
	h, err := meter.ParseBytes32(value["value"])
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, meter.BytesToBytes32([]byte{storageValue}), h, "storage should be equal")
	assert.Equal(t, http.StatusOK, statusCode, "OK")
}

func initAccountServer(t *testing.T) {
	meter.InitBlockChainConfig("test")
	db, _ := lvldb.NewMem()
	stateC := state.NewCreator(db)
	gene := genesis.NewDevnet()

	b, _, err := gene.Build(stateC)
	if err != nil {
		t.Fatal(err)
	}
	chain, _ := chain.New(db, b, false)
	claTransfer := tx.NewClause(&addr).WithValue(value)
	claDeploy := tx.NewClause(nil).WithData(bytecode)
	transaction := buildTxWithClauses(t, chain.Tag(), claTransfer, claDeploy)
	contractAddr = meter.Address(meter.EthCreateContractAddress(common.Address(genesis.DevAccounts()[0].Address), uint32(transaction.Nonce()+1)))
	packTx(chain, stateC, transaction, t)

	method := "set"
	abi, err := ABI.New([]byte(abiJSON))
	m, _ := abi.MethodByName(method)
	input, err := m.EncodeInput(uint8(storageValue))
	if err != nil {
		t.Fatal(err)
	}
	claCall := tx.NewClause(&contractAddr).WithData(input)
	transactionCall := buildTxWithClauses(t, chain.Tag(), claCall)
	packTx(chain, stateC, transactionCall, t)

	router := mux.NewRouter()
	accounts.New(chain, stateC, math.MaxUint64).Mount(router, "/accounts")
	ts = httptest.NewServer(router)
}

func buildTxWithClauses(t *testing.T, chaiTag byte, clauses ...*tx.Clause) *tx.Transaction {
	builder := new(tx.Builder).
		ChainTag(chaiTag).
		Expiration(10).
		Gas(1000000)
	for _, c := range clauses {
		builder.Clause(c)
	}

	transaction := builder.Build()
	sig, err := crypto.Sign(transaction.SigningHash().Bytes(), genesis.DevAccounts()[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return transaction.WithSignature(sig)
}

func packTx(chain *chain.Chain, stateC *state.Creator, transaction *tx.Transaction, t *testing.T) {
	b := chain.BestBlock()
	p := packer.New(chain, stateC, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address)
	flow, err := p.Mock(b.Header(), uint64(time.Now().Unix()), 2000000, &meter.Address{})
	err = flow.Adopt(transaction)
	if err != nil {
		t.Fatal(err)
	}
	b, stage, receipts, err := flow.Pack(genesis.DevAccounts()[0].PrivateKey, block.MBlockType, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Commit(); err != nil {
		t.Fatal(err)
	}
	b.SetQC(&block.QuorumCert{QCHeight: 1, QCRound: 1, EpochID: 1})
	escortQC := &block.QuorumCert{QCHeight: b.Number(), QCRound: b.QC.QCRound + 1, EpochID: b.QC.EpochID, VoterMsgHash: b.VotingHash()}
	if _, err := chain.AddBlock(b, escortQC, receipts); err != nil {
		t.Fatal(err)
	}
}

func deployContractWithCall(t *testing.T) {
	badBody := &accounts.CallData{
		Gas:  10000000,
		Data: "abc",
	}
	_, statusCode := httpPost(t, ts.URL+"/accounts/*", badBody)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad data")

	reqBody := &accounts.CallData{
		Gas:  10000000,
		Data: hexutil.Encode(bytecode),
	}

	_, statusCode = httpPost(t, ts.URL+"/accounts/*?revision="+invalidNumberRevision, reqBody)
	assert.Equal(t, http.StatusBadRequest, statusCode, "bad revision")

	//revision is optional defaut `best`
	// res, statusCode = httpPost(t, ts.URL+"/accounts", reqBody)
	// var output *accounts.CallResult
	// fmt.Println(string(res))
	// if err := json.Unmarshal(res, &output); err != nil {
	// 	t.Fatal(err)
	// }
	// assert.False(t, output.Reverted)

}

func callContract(t *testing.T) {
	res, statusCode := httpPost(t, ts.URL+"/accounts/"+invalidAddr, nil)
	assert.Equal(t, http.StatusBadRequest, statusCode, "invalid address")

	badBody := &accounts.CallData{
		Data: "input",
	}
	res, statusCode = httpPost(t, ts.URL+"/accounts/"+contractAddr.String(), badBody)
	assert.Equal(t, http.StatusBadRequest, statusCode, "invalid input data")

	a := uint8(1)
	b := uint8(2)
	method := "add"
	abi, err := ABI.New([]byte(abiJSON))
	m, _ := abi.MethodByName(method)
	input, err := m.EncodeInput(a, b)
	if err != nil {
		t.Fatal(err)
	}
	reqBody := &accounts.CallData{
		Data: hexutil.Encode(input),
	}
	res, statusCode = httpPost(t, ts.URL+"/accounts/"+contractAddr.String(), reqBody)
	var output *accounts.CallResult
	if err = json.Unmarshal(res, &output); err != nil {
		t.Fatal(err)
	}
	data, err := hexutil.Decode(output.Data)
	if err != nil {
		t.Fatal(err)
	}
	var ret uint8
	err = m.DecodeOutput(data, &ret)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, a+b, ret)
}

func batchCall(t *testing.T) {
	badBody := &accounts.BatchCallData{
		Clauses: accounts.Clauses{
			accounts.Clause{
				To:    &contractAddr,
				Data:  "data1",
				Value: nil,
			},
			accounts.Clause{
				To:    &contractAddr,
				Data:  "data2",
				Value: nil,
			}},
	}
	res, statusCode := httpPost(t, ts.URL+"/accounts/*", badBody)
	assert.Equal(t, http.StatusBadRequest, statusCode, "invalid data")

	a := uint8(1)
	b := uint8(2)
	method := "add"
	abi, err := ABI.New([]byte(abiJSON))
	m, _ := abi.MethodByName(method)
	input, err := m.EncodeInput(a, b)
	if err != nil {
		t.Fatal(err)
	}
	reqBody := &accounts.BatchCallData{
		Clauses: accounts.Clauses{
			accounts.Clause{
				To:    &contractAddr,
				Data:  hexutil.Encode(input),
				Value: nil,
			},
			accounts.Clause{
				To:    &contractAddr,
				Data:  hexutil.Encode(input),
				Value: nil,
			}},
	}

	res, statusCode = httpPost(t, ts.URL+"/accounts/*?revision="+invalidNumberRevision, badBody)
	assert.Equal(t, http.StatusBadRequest, statusCode, "invalid revision")

	res, statusCode = httpPost(t, ts.URL+"/accounts/*", reqBody)
	var results accounts.BatchCallResults
	if err = json.Unmarshal(res, &results); err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		data, err := hexutil.Decode(result.Data)
		if err != nil {
			t.Fatal(err)
		}
		var ret uint8
		err = m.DecodeOutput(data, &ret)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, a+b, ret, "should be equal")
	}
	assert.Equal(t, http.StatusOK, statusCode)
}

func httpPost(t *testing.T, url string, body interface{}) ([]byte, int) {
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(url, "application/x-www-form-urlencoded", bytes.NewReader(data))
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
