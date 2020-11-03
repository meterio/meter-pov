// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package transferslegacy_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dfinlab/meter/api/transferslegacy"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var ts *httptest.Server

func TestTransfers(t *testing.T) {
	initLogServer(t)
	defer ts.Close()
	getTransfers(t)
}

func getTransfers(t *testing.T) {
	limit := 5
	from := meter.BytesToAddress([]byte("from"))
	to := meter.BytesToAddress([]byte("to"))
	tf := &transferslegacy.TransferFilter{
		AddressSets: []*transferslegacy.AddressSet{
			&transferslegacy.AddressSet{
				TxOrigin:  &from,
				Recipient: &to,
			},
		},
		Range: &logdb.Range{
			Unit: logdb.Block,
			From: 0,
			To:   1000,
		},
		Options: &logdb.Options{
			Offset: 0,
			Limit:  uint64(limit),
		},
		Order: logdb.DESC,
	}
	res := httpPost(t, ts.URL+"/logs/transfers", tf)
	var tLogs []*transferslegacy.FilteredTransfer
	if err := json.Unmarshal(res, &tLogs); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, limit, len(tLogs), "should be `limit` transfers")
}

func initLogServer(t *testing.T) {
	db, err := logdb.NewMem()
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
		if err := db.Prepare(header).ForTransaction(meter.Bytes32{}, from).Insert(nil, tx.Transfers{transLog}).
			Commit(); err != nil {
			t.Fatal(err)
		}
	}

	router := mux.NewRouter()
	transferslegacy.New(db).Mount(router, "/logs/transfers")
	ts = httptest.NewServer(router)
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
	r, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return r
}
