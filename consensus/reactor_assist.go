package consensus

import (
	"math/big"
	"net"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/types"
)

// build block committee info part
func (r *Reactor) MakeBlockCommitteeInfo() []block.CommitteeInfo {
	system := r.blsCommon.GetSystem()
	cms := r.committee

	cis := []block.CommitteeInfo{}

	for index, cm := range cms {
		ci := block.NewCommitteeInfo(cm.Name, crypto.FromECDSAPub(&cm.PubKey), cm.NetAddr,
			system.PubKeyToBytes(cm.BlsPubKey), uint32(index))
		cis = append(cis, *ci)
	}
	return (cis)
}

func convertDistList(dist []*meter.Distributor) []*types.Distributor {
	list := []*types.Distributor{}
	for _, d := range dist {
		l := &types.Distributor{
			Address: d.Address,
			Autobid: d.Autobid,
			Shares:  d.Shares,
		}
		list = append(list, l)
	}
	return list
}

func (r *Reactor) getDelegatesFromStaking() ([]*types.Delegate, error) {
	delegateList := []*types.Delegate{}

	best := r.chain.BestBlock()
	state, err := r.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		return delegateList, err
	}

	list := state.GetDelegateList()
	r.logger.Debug("Loaded delegateList from staking", "len", len(list.Delegates))
	for _, s := range list.Delegates {
		pubKey, blsPub := r.splitPubKey(string(s.PubKey))
		d := &types.Delegate{
			Name:        s.Name,
			Address:     s.Address,
			PubKey:      *pubKey,
			BlsPubKey:   *blsPub,
			VotingPower: new(big.Int).Div(s.VotingPower, big.NewInt(1e12)).Int64(),
			Commission:  s.Commission,
			NetAddr: types.NetAddress{
				IP:   net.ParseIP(string(s.IPAddr)),
				Port: s.Port},
			DistList: convertDistList(s.DistList),
		}
		d.SetInternCombinePublicKey(string(s.PubKey))
		delegateList = append(delegateList, d)
	}
	return delegateList, nil
}
