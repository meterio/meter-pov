// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.htl>

package staking

import (
	"encoding/hex"
	"math"
	"net/http"
	"strconv"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/state"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type Staking struct {
	chain        *chain.Chain
	stateCreator *state.Creator
}

func New(chain *chain.Chain,
	stateCreator *state.Creator) *Staking {
	return &Staking{chain: chain, stateCreator: stateCreator}
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
	converted := convertBucket(bucket)
	return utils.WriteJSON(w, converted)
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

func (st *Staking) handleGetLastValidatorReward(w http.ResponseWriter, req *http.Request) error {
	h, err := st.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	list, err := staking.GetValidatorRewardListByHeader(h)
	if err != nil {
		return err
	}
	last := list.Last()
	if last == nil {
		last = &staking.ValidatorReward{}
	}
	reward := convertValidatorReward(*last)
	return utils.WriteJSON(w, reward)
}

func (st *Staking) handleGetValidatorRewardList(w http.ResponseWriter, req *http.Request) error {
	h, err := st.handleRevision(req.URL.Query().Get("revision"))
	if err != nil {
		return err
	}
	list, err := staking.GetValidatorRewardListByHeader(h)
	if err != nil {
		return err
	}
	validatorRewardList := convertValidatorRewardList(list)
	return utils.WriteJSON(w, validatorRewardList)
}

func (st *Staking) handleRevision(revision string) (*block.Header, error) {
	if revision == "" || revision == "best" {
		return st.chain.BestBlock().Header(), nil
	}
	if len(revision) == 66 || len(revision) == 64 {
		blockID, err := meter.ParseBytes32(revision)
		if err != nil {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		h, err := st.chain.GetBlockHeader(blockID)
		if err != nil {
			if st.chain.IsNotFound(err) {
				return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
			}
			return nil, err
		}
		return h, nil
	}
	n, err := strconv.ParseUint(revision, 0, 0)
	if err != nil {
		return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
	}
	if n > math.MaxUint32 {
		return nil, utils.BadRequest(errors.WithMessage(errors.New("block number out of max uint32"), "revision"))
	}
	h, err := st.chain.GetTrunkBlockHeader(uint32(n))
	if err != nil {
		if st.chain.IsNotFound(err) {
			return nil, utils.BadRequest(errors.WithMessage(err, "revision"))
		}
		return nil, err
	}
	return h, nil
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
	sub.Path("/last/rewards").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetLastValidatorReward))
}
