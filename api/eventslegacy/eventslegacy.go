// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package eventslegacy

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/meter"
	"github.com/pkg/errors"
)

var (
	log = log15.New("pkg", "evtlgyapi")
)

type EventsLegacy struct {
	db *logdb.LogDB
}

func New(db *logdb.LogDB) *EventsLegacy {
	return &EventsLegacy{
		db,
	}
}

// Filter query events with option
func (e *EventsLegacy) filter(ctx context.Context, filter *FilterLegacy) ([]*FilteredEvent, error) {
	start := time.Now()
	f := convertFilter(filter)
	events, err := e.db.FilterEvents(ctx, f)
	if err != nil {
		return nil, err
	}
	fes := make([]*FilteredEvent, len(events))
	for i, e := range events {
		fes[i] = convertEvent(e)
	}
	if time.Since(start) > time.Second {
		log.Info("slow filter event legacy ", "elapsed", meter.PrettyDuration(time.Since(start)))
	}
	return fes, nil
}

func (e *EventsLegacy) handleFilter(w http.ResponseWriter, req *http.Request) error {
	var filter FilterLegacy
	start := time.Now()
	if err := utils.ParseJSON(req.Body, &filter); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	query := req.URL.Query()
	if query.Get("address") != "" {
		as := query["address"]

		addrs := []*meter.Address{}
		for _, a := range as {
			addr, err := meter.ParseAddress(a)
			if err != nil {
				return utils.BadRequest(errors.WithMessage(err, "address"))
			}
			addrs = append(addrs, &addr)
		}
		filter.Address = addrs
	}
	order := query.Get("order")
	if order != string(logdb.DESC) {
		filter.Order = logdb.ASC
	} else {
		filter.Order = logdb.DESC
	}
	fes, err := e.filter(req.Context(), &filter)
	if err != nil {
		return err
	}
	filterStr, err := json.Marshal(filter)
	err = utils.WriteJSON(w, fes)
	if time.Since(start) > time.Second {
		log.Info("handle event legacy", "filter", string(filterStr), "elapsed", meter.PrettyDuration(time.Since(start)))
	}
	return err
}

func (e *EventsLegacy) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(e.handleFilter))
}
