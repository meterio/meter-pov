// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package events_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dfinlab/meter/api/events"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var contractAddr = meter.BytesToAddress([]byte("contract"))
var ts *httptest.Server

func TestEvents(t *testing.T) {
	initEventServer(t)
	defer ts.Close()
	getEvents(t)
}

func getEvents(t *testing.T) {
	t0 := meter.BytesToBytes32([]byte("topic0"))
	t1 := meter.BytesToBytes32([]byte("topic1"))
	limit := 5
	filter := &events.EventFilter{
		Range: &logdb.Range{
			Unit: "",
			From: 0,
			To:   10,
		},
		Options: &logdb.Options{
			Offset: 0,
			Limit:  uint64(limit),
		},
		Order: "",
		CriteriaSet: []*events.EventCriteria{
			&events.EventCriteria{
				Address: &contractAddr,
				TopicSet: events.TopicSet{
					Topic0: &t0,
				},
			},
			&events.EventCriteria{
				Address: &contractAddr,
				TopicSet: events.TopicSet{
					Topic1: &t1,
				},
			},
		},
	}
	res := httpPost(t, ts.URL+"/logs/event?", filter)
	var logs []*events.FilteredEvent
	if err := json.Unmarshal(res, &logs); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, limit, len(logs), "should be `limit` logs")
}
func initEventServer(t *testing.T) {
	db, err := logdb.NewMem()
	if err != nil {
		t.Fatal(err)
	}
	txEv := &tx.Event{
		Address: contractAddr,
		Topics:  []meter.Bytes32{meter.BytesToBytes32([]byte("topic0")), meter.BytesToBytes32([]byte("topic1"))},
		Data:    []byte("data"),
	}

	header := new(block.Builder).Build().Header()
	for i := 0; i < 100; i++ {
		if err := db.Prepare(header).ForTransaction(meter.BytesToBytes32([]byte("txID")), meter.BytesToAddress([]byte("txOrigin"))).
			Insert(tx.Events{txEv}, nil).Commit(); err != nil {
			if err != nil {
				t.Fatal(err)
			}
		}
		header = new(block.Builder).ParentID(header.ID()).Build().Header()
	}

	router := mux.NewRouter()
	events.New(db).Mount(router, "/logs/event")
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
