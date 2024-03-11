// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package transactions_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/transactions"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
	"github.com/stretchr/testify/assert"
)

var c *chain.Chain
var ts *httptest.Server
var transaction *tx.Transaction

func TestTransaction(t *testing.T) {
	initTransactionServer(t)
	defer ts.Close()
	getTx(t)
	getTxReceipt(t)
	senTx(t)
}

func getTx(t *testing.T) {
	res := httpGet(t, ts.URL+"/transactions/"+transaction.ID().String())
	var rtx *transactions.Transaction
	if err := json.Unmarshal(res, &rtx); err != nil {
		t.Fatal(err)
	}
	checkTx(t, transaction, rtx)

	res = httpGet(t, ts.URL+"/transactions/"+transaction.ID().String()+"?raw=true")
	var rawTx map[string]interface{}
	if err := json.Unmarshal(res, &rawTx); err != nil {
		t.Fatal(err)
	}
	rlpTx, err := rlp.EncodeToBytes(transaction)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, hexutil.Encode(rlpTx), rawTx["raw"], "should be equal raw")
}

func getTxReceipt(t *testing.T) {
	r := httpGet(t, ts.URL+"/transactions/"+transaction.ID().String()+"/receipt")
	var receipt *transactions.Receipt
	if err := json.Unmarshal(r, &receipt); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, uint64(receipt.GasUsed), transaction.Gas(), "gas should be equal")
}

func senTx(t *testing.T) {
	var blockRef = tx.NewBlockRef(0)
	var chainTag = c.Tag()
	var expiration = uint32(10)
	var gas = uint64(21000)

	tx := new(tx.Builder).
		BlockRef(blockRef).
		ChainTag(chainTag).
		Expiration(expiration).
		Gas(gas).
		Build()
	sig, err := crypto.Sign(tx.SigningHash().Bytes(), genesis.DevAccounts()[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	tx = tx.WithSignature(sig)
	rlpTx, err := rlp.EncodeToBytes(tx)
	if err != nil {
		t.Fatal(err)
	}

	res := httpPost(t, ts.URL+"/transactions", transactions.RawTx{Raw: hexutil.Encode(rlpTx)})
	var txObj map[string]string
	if err = json.Unmarshal(res, &txObj); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tx.ID().String(), txObj["id"], "should be the same transaction id")

	unsignedTx := transactions.UnSignedTx{
		ChainTag:   chainTag,
		BlockRef:   hexutil.Encode(blockRef[:]),
		Expiration: expiration,
		Gas:        gas,
	}
	res = httpPost(t, ts.URL+"/transactions", unsignedTx)
	if err = json.Unmarshal(res, &txObj); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tx.SigningHash().String(), txObj["signingHash"], "should be the same transaction signingHash")

	signedTx := transactions.SignedTx{
		UnSignedTx: unsignedTx,
		Signature:  hexutil.Encode(sig),
	}
	res = httpPost(t, ts.URL+"/transactions", signedTx)
	if err = json.Unmarshal(res, &txObj); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, tx.ID().String(), txObj["id"], "should be the same transaction id")
}

func httpPost(t *testing.T, url string, obj interface{}) []byte {
	data, err := json.Marshal(obj)
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
	return r
}

func initTransactionServer(t *testing.T) {
	meter.InitBlockChainConfig("test")
	logDB, err := logdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	from := meter.BytesToAddress([]byte("from"))
	to := meter.BytesToAddress([]byte("to"))
	value := big.NewInt(10)
	header := new(block.Builder).Build().Header()
	count := 100
	for i := 0; i < count; i++ {
		transLog := &tx.Transfer{
			Sender:    from,
			Recipient: to,
			Amount:    value,
		}
		header = new(block.Builder).ParentID(header.ID()).Build().Header()
		if err := logDB.Prepare(header).ForTransaction(meter.Bytes32{}, from).
			Insert(nil, tx.Transfers{transLog}).Commit(); err != nil {
			t.Fatal(err)
		}
	}
	db, _ := lvldb.NewMem()
	stateC := state.NewCreator(db)
	gene := genesis.NewDevnet()

	b, _, err := gene.Build(stateC)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = chain.New(db, b, false)
	addr := meter.BytesToAddress([]byte("to"))
	cla := tx.NewClause(&addr).WithValue(big.NewInt(10000))
	transaction = new(tx.Builder).
		ChainTag(c.Tag()).
		GasPriceCoef(1).
		Expiration(10).
		Gas(21000).
		Nonce(1).
		Clause(cla).
		BlockRef(tx.NewBlockRef(0)).
		Build()

	sig, err := crypto.Sign(transaction.SigningHash().Bytes(), genesis.DevAccounts()[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	transaction = transaction.WithSignature(sig)
	packer := packer.New(c, stateC, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address)
	flow, err := packer.Mock(b.Header(), uint64(time.Now().Unix()), 2000000, &meter.Address{})
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
	b.SetQC(&block.QuorumCert{QCHeight: 1, QCRound: 1, EpochID: 0})
	escortQC := &block.QuorumCert{QCHeight: b.Number(), QCRound: b.QC.QCRound + 1, EpochID: b.QC.EpochID, VoterMsgHash: b.VotingHash()}
	if _, err := c.AddBlock(b, escortQC, receipts); err != nil {
		t.Fatal(err)
	}
	router := mux.NewRouter()
	transactions.New(c, stateC, txpool.New(c, stateC, txpool.Options{Limit: 10000, LimitPerAccount: 16, MaxLifetime: 10 * time.Minute})).Mount(router, "/transactions")
	ts = httptest.NewServer(router)

}

func checkTx(t *testing.T, expectedTx *tx.Transaction, actualTx *transactions.Transaction) {
	origin, err := expectedTx.Signer()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, origin, actualTx.Origin)
	assert.Equal(t, expectedTx.ID(), actualTx.ID)
	assert.Equal(t, expectedTx.GasPriceCoef(), actualTx.GasPriceCoef)
	assert.Equal(t, expectedTx.Gas(), actualTx.Gas)
	for i, c := range expectedTx.Clauses() {
		assert.Equal(t, hexutil.Encode(c.Data()), actualTx.Clauses[i].Data)
		assert.Equal(t, *c.Value(), big.Int(actualTx.Clauses[i].Value))
		assert.Equal(t, c.To(), actualTx.Clauses[i].To)
	}

}

func httpGet(t *testing.T, url string) []byte {
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	r, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return r
}
