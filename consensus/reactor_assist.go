package consensus

import (
	"math/big"
	"net"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/types"
)

// build block committee info part
func (r *Reactor) MakeBlockCommitteeInfo() []block.CommitteeInfo {
	cis := []block.CommitteeInfo{}

	for index, cm := range r.committee {
		ci := block.NewCommitteeInfo(cm.Name, cm.PubKeyBytes, cm.NetAddr,
			cm.BlsPubKeyBytes, uint32(index))
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

func (r *Reactor) getDelegatesFromStaking(revision *block.Block) ([]*types.Delegate, error) {
	delegateList := []*types.Delegate{}

	state, err := r.stateCreator.NewState(revision.StateRoot())
	if err != nil {
		return delegateList, err
	}

	list := state.GetDelegateList()
	r.logger.Debug("Loaded delegateList from staking", "len", len(list.Delegates))
	for _, s := range list.Delegates {
		pubKey, blsPub := r.blsCommon.SplitPubKey(string(s.PubKey))

		d := types.NewDelegate([]byte(s.Name), s.Address, *pubKey, *blsPub, string(s.PubKey), new(big.Int).Div(s.VotingPower, big.NewInt(1e12)).Int64(), s.Commission, types.NetAddress{
			IP:   net.ParseIP(string(s.IPAddr)),
			Port: s.Port})
		d.DistList = convertDistList(s.DistList)
		delegateList = append(delegateList, d)
	}
	return delegateList, nil
}
