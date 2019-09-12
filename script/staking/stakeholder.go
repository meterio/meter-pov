package staking

import (
	"io"
	"math/big"

	"github.com/dfinlab/geth-zDollar/rlp"
	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

var (
	StakeholderMap map[meter.Address]*Stakeholder
)

// Stakeholder indicates the structure of a Stakeholder
type Stakeholder struct {
	Holder     meter.Address // the address for staking / reward
	TotalStake *big.Int      // total voting from all buckets
	Buckets    []uuid.UUID   // all buckets voted for this Stakeholder
}

func NewStakeholder(holder meter.Address) *Stakeholder {
	return &Stakeholder{
		Holder:     holder,
		TotalStake: big.NewInt(0),
		Buckets:    []uuid.UUID{},
	}
}

func StakeholderListToMap(StakeholderList []Stakeholder) error {
	for _, c := range StakeholderList {
		StakeholderMap[c.Holder] = &c
	}
	return nil
}

func StakeholderMapToList() ([]Stakeholder, error) {
	StakeholderList := []Stakeholder{}
	for _, c := range StakeholderMap {
		StakeholderList = append(StakeholderList, *c)
	}
	return StakeholderList, nil
}

// EncodeRLP implements rlp.Encoder.
func (s *Stakeholder) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		s.Holder,
		s.TotalStake,
		s.Buckets,
	})
}

// DecodeRLP implements rlp.Decoder.
func (s *Stakeholder) DecodeRLP(stream *rlp.Stream) error {
	payload := struct {
		Holder     meter.Address
		TotalStake *big.Int
		Buckets    []uuid.UUID
	}{}

	if err := stream.Decode(&payload); err != nil {
		return err
	}

	*s = Stakeholder{
		Holder:     payload.Holder,
		TotalStake: payload.TotalStake,
		Buckets:    payload.Buckets,
	}
	return nil
}

func (s *Stakeholder) Add()    {}
func (s *Stakeholder) Update() {}
func (s *Stakeholder) Remove() {}
