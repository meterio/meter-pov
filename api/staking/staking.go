// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package staking

import (
	"encoding/hex"
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
	"github.com/gorilla/mux"
)

type Staking struct {
}

func New() *Staking {
	return &Staking{}
}

func (st *Staking) handleGetCandidateList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestCandidateList()
	if err != nil {
		return err
	}
	candidateList := convertCandidateList(list)
	return utils.WriteJSON(w, candidateList)
}

func (st *Staking) handleGetCandidateByAddress(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestCandidateList()
	if err != nil {
		return err
	}
	addr := mux.Vars(req)["address"]
	bytes, err := hex.DecodeString(addr)
	if err != nil {
		return err
	}
	meterAddr := meter.BytesToAddress(bytes)
	c := list.Get(meterAddr)
	candidate := convertCandidate(*c)
	return utils.WriteJSON(w, candidate)
}

func (st *Staking) handleGetBucketList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestBucketList()
	if err != nil {
		return err
	}
	bucketList := convertBucketList(list)

	return utils.WriteJSON(w, bucketList)
}

func (st *Staking) handleGetBucketByID(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestBucketList()
	id := mux.Vars(req)["id"]
	bucketID, err := meter.ParseBytes32(id)
	if err != nil {
		return err
	}
	bucket := list.Get(bucketID)
	return utils.WriteJSON(w, bucket)
}

func (st *Staking) handleGetStakeholderList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestStakeholderList()
	if err != nil {
		return err
	}
	bucketList := convertStakeholderList(list)

	return utils.WriteJSON(w, bucketList)
}

func (st *Staking) handleGetStakeholderByAddress(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestStakeholderList()
	addr := mux.Vars(req)["address"]
	bytes, err := hex.DecodeString(addr)
	if err != nil {
		return err
	}
	meterAddr := meter.BytesToAddress(bytes)

	s := list.Get(meterAddr)
	stakeholder := convertStakeholder(*s)
	return utils.WriteJSON(w, stakeholder)
}

func (st *Staking) handleGetDelegateList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestDelegateList()
	if err != nil {
		return err
	}
	delegateList := convertDelegateList(list)
	return utils.WriteJSON(w, delegateList)
}

func (st *Staking) handleGetValidatorRewardList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.GetLatestValidatorRewardList()
	if err != nil {
		return err
	}
	validatorRewardList := convertValidatorRewardList(list)
	return utils.WriteJSON(w, validatorRewardList)
}

func (st *Staking) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/candidates").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetCandidateList))
	sub.Path("/buckets").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetBucketList))
	sub.Path("/buckets/{id}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetBucketByID))
	sub.Path("/candidates/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetCandidateByAddress))
	sub.Path("/stakeholders").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetStakeholderList))
	sub.Path("/stakeholders/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetStakeholderByAddress))
	sub.Path("/delegates").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetDelegateList))
	sub.Path("/validator-rewards").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetValidatorRewardList))
}
