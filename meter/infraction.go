package meter

import "fmt"

// MissingLeader
type MissingLeaderInfo struct {
	Epoch uint32
	Round uint32
}
type MissingLeader struct {
	Counter uint32
	Info    []*MissingLeaderInfo
}

// MissingProposer
type MissingProposerInfo struct {
	Epoch  uint32
	Height uint32
}
type MissingProposer struct {
	Counter uint32
	Info    []*MissingProposerInfo
}

func (m MissingProposer) String() string {
	str := ""
	for _, i := range m.Info {
		str += fmt.Sprintf("(E:%d, H:%d)", i.Epoch, i.Height)
	}
	return fmt.Sprintf("[%d %s]", m.Counter, str)
}

// MissingVoter
type MissingVoterInfo struct {
	Epoch  uint32
	Height uint32
}
type MissingVoter struct {
	Counter uint32
	Info    []*MissingVoterInfo
}

// DoubleSigner
type DoubleSignerInfo struct {
	Epoch  uint32
	Height uint32
}
type DoubleSigner struct {
	Counter uint32
	Info    []*DoubleSignerInfo
}

type Infraction struct {
	MissingLeaders   MissingLeader
	MissingProposers MissingProposer
	MissingVoters    MissingVoter
	DoubleSigners    DoubleSigner
}

func (inf *Infraction) String() string {
	if inf == nil {
		return "infraction(nil)"
	}
	return fmt.Sprintf("Infraction(leader:%v, proposer:%v, voter:%v, doubleSign:%v)", inf.MissingLeaders, inf.MissingProposers, inf.MissingVoters, inf.DoubleSigners)
}
