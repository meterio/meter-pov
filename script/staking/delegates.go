package staking

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	crypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/types"
)

type Delegate struct {
	Address     meter.Address
	PubKey      []byte //ecdsa.PublicKey
	Name        []byte
	VotingPower *big.Int
	IPAddr      []byte
	Port        uint16
}

type DelegateList struct {
	delegates []*Delegate
}

func newDelegateList(delegates []*Delegate) *DelegateList {
	return &DelegateList{delegates: delegates}
}

func (d *Delegate) ToString() string {
	return fmt.Sprintf("Delegate(Addr=%v, PubKey=%v, IP:Port=%v:%v, VotingPower=%.2e)",
		d.Address, hex.EncodeToString(d.PubKey), string(d.IPAddr), d.Port, float64(d.VotingPower.Int64()))
}

func (l *DelegateList) CleanAll() error {
	l.delegates = []*Delegate{}
	return nil
}

func (l *DelegateList) SetDelegates(delegates []*Delegate) error {
	l.delegates = delegates
	return nil
}

func (l *DelegateList) Add(c *Delegate) error {
	l.delegates = append(l.delegates, c)
	return nil
}

func (l *DelegateList) ToString() string {
	s := []string{fmt.Sprintf("DelegateList (size:%v):", len(l.delegates))}
	for k, v := range l.delegates {
		s = append(s, fmt.Sprintf("%v. %v", k, v.ToString()))
	}
	s = append(s, "")
	return strings.Join(s, "\n")
}

//  api routine interface
func GetLatestDelegateList() ([]*types.Delegate, error) {
	fmt.Println("get into GetLatestDelegateList")
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
	fmt.Println("delegateList from state", list.ToString())
	for _, s := range list.delegates {
		pubKey, err := crypto.UnmarshalPubkey(s.PubKey)
		if err != nil {
			fmt.Println("Unmarshal publicKey failed ...")
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
