package staking

import (
	"net/http"

	"github.com/dfinlab/meter/api/utils"
	"github.com/dfinlab/meter/script/staking"
	"github.com/gorilla/mux"
)

type Staking struct {
}

func New() *Staking {
	return &Staking{}
}

func (st *Staking) handleGetCandidateList(w http.ResponseWriter, req *http.Request) error {
	candidateList, err := st.getCandidateList()
	if err != nil {
		return err
	}
	return utils.WriteJSON(w, candidateList)
}

func (st *Staking) handleGetCandidateByAddress(w http.ResponseWriter, req *http.Request) error {
	addr := mux.Vars(req)["address"]

	candidateList, err := st.getCandidateList()
	if err != nil {
		return err
	}

	for _, c := range candidateList {
		if c.Addr.String() == addr {
			return utils.WriteJSON(w, c)
		}
	}
	return utils.WriteJSON(w, nil)
}

func (st *Staking) getCandidateList() (candidateList []Candidate, err error) {
	list, err := staking.CandidateMapToList()
	if err != nil {
		return nil, err
	}
	candidateList = convertCandidatesList(list)
	return candidateList, nil
}

func (st *Staking) handleGetBucketList(w http.ResponseWriter, req *http.Request) error {
	list, err := staking.BucketMapToList()
	if err != nil {
		return err
	}
	bucketList := convertBucketList(list)

	return utils.WriteJSON(w, bucketList)
}

func (st *Staking) Mount(root *mux.Router, pathPrefix string) {
	sub := root.PathPrefix(pathPrefix).Subrouter()
	sub.Path("/candidates").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetCandidateList))
	sub.Path("/buckets").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetBucketList))
	sub.Path("/candidates/{address}").Methods("Get").HandlerFunc(utils.WrapHandlerFunc(st.handleGetCandidateByAddress))
}
