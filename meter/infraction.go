package meter

import (
	"fmt"
	"strings"
)

// MissingLeader
type MissingLeaderInfo struct {
	Epoch uint32
	Round uint32
}
type MissingLeader struct {
	Counter uint32
	Info    []*MissingLeaderInfo
}

func (m MissingLeader) String() string {
	if m.Counter == 0 {
		return ""
	}
	s := make([]string, 0)
	for _, i := range m.Info {
		s = append(s, fmt.Sprintf("(E:%d, R:%d)", i.Epoch, i.Round))
	}
	return "missingLeader:" + strings.Join(s, ",")
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
	if m.Counter == 0 {
		return ""
	}
	s := make([]string, 0)
	for _, i := range m.Info {
		s = append(s, fmt.Sprintf("(E:%d, #%d)", i.Epoch, i.Height))
	}
	return "missingProposer:" + strings.Join(s, ",")
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

func (m MissingVoter) String() string {
	if m.Counter == 0 {
		return ""
	}
	s := make([]string, 0)
	for _, i := range m.Info {
		s = append(s, fmt.Sprintf("(E:%d, #%d)", i.Epoch, i.Height))
	}
	return "missingVoter:" + strings.Join(s, ",")
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

func (m DoubleSigner) String() string {
	if m.Counter == 0 {
		return ""
	}
	s := make([]string, 0)
	for _, i := range m.Info {
		s = append(s, fmt.Sprintf("(E:%d, #%d)", i.Epoch, i.Height))
	}
	return "doubleSign:" + strings.Join(s, ",")
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
	s := []string{inf.MissingLeaders.String(), inf.MissingProposers.String(), inf.MissingVoters.String(), inf.DoubleSigners.String()}
	nonempty := []string{}
	for _, str := range s {
		if str != "" {
			nonempty = append(nonempty, str)
		}
	}
	return fmt.Sprintf("Infraction(%v)", strings.Join(nonempty, ","))
}
