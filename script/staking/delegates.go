package staking

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	crypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/types"
)

type SDelegate struct {
	Address     meter.Address
	PubKey      []byte //ecdsa.PublicKey
	Name        []byte
	VotingPower *big.Int
	IPAddr      []byte
	Port        uint16
}

// periodically change candidates to delegates
func CandidatesToDelegates(size int) error {
	staking := GetStakingGlobInst()
	if staking == nil {
		fmt.Println("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return err
	}

	candList := staking.GetCandidateList(state)
	delegateList := []SDelegate{}

	for _, c := range candList.candidates {
		d := SDelegate{
			Address:     c.Addr,
			PubKey:      c.PubKey,
			Name:        c.Name,
			VotingPower: c.TotalVotes,
			IPAddr:      c.IPAddr,
			Port:        c.Port,
		}
		delegateList = append(delegateList, d)
	}

	sort.SliceStable(delegateList, func(i, j int) bool {
		return (delegateList[i].VotingPower.Cmp(delegateList[j].VotingPower) <= 0)
	})

	staking.SetDelegateList(delegateList[:size], state)
	return nil
}

//  api routine interface
func GetLatestDelegateList() ([]*types.Delegate, error) {
	delegateList := []*types.Delegate{}
	staking := GetStakingGlobInst()
	if staking == nil {
		fmt.Println("staking is not initilized...")
		err := errors.New("staking is not initilized...")
		return delegateList, err
	}

	best := staking.chain.BestBlock()
	state, err := staking.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return delegateList, err
	}

	list := staking.GetDelegateList(state)
	for _, s := range list {
		pubKey, err := crypto.UnmarshalPubkey(s.PubKey)
		if err != nil {
			fmt.Println("Unmarshal publicKey failed")
			continue
		}

		d := &types.Delegate{
			Address:     s.Address,
			PubKey:      *pubKey,
			VotingPower: s.VotingPower.Int64(),
			NetAddr: types.NetAddress{
				IP:   s.IPAddr,
				Port: s.Port},
		}
		delegateList = append(delegateList, d)
	}
	return delegateList, nil
}
