// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package events

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/meterio/meter-pov/api/utils"
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/meter"
	"github.com/pkg/errors"
)

type Events struct {
	db *logdb.LogDB
}

func New(db *logdb.LogDB) *Events {
	return &Events{
		db,
	}
}

// Filter query events with option
func (e *Events) filter(ctx context.Context, ef *EventFilter) ([]*FilteredEvent, error) {
	events, err := e.db.FilterEvents(ctx, convertEventFilter(ef))
	if err != nil {
		return nil, err
	}
	fes := make([]*FilteredEvent, len(events))
	for i, e := range events {
		fes[i] = convertEvent(e)
	}
	return fes, nil
}

func (e *Events) handleFilter(w http.ResponseWriter, req *http.Request) error {
	start := time.Now()
	var filter EventFilter
	if err := utils.ParseJSON(req.Body, &filter); err != nil {
		return utils.BadRequest(errors.WithMessage(err, "body"))
	}
	fes, err := e.filter(req.Context(), &filter)
	if err != nil {
		return err
	}
	filterStr, err := json.Marshal(filter)
	err = utils.WriteJSON(w, fes)

	if time.Since(start) > time.Second {
		log.Info("slow handled event query", "query", string(filterStr), "elapsed", meter.PrettyDuration(time.Since(start)))
	}
	return err
}

func (e *Events) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()

	sub.Path("").Methods("POST").HandlerFunc(utils.WrapHandlerFunc(e.handleFilter))
}
