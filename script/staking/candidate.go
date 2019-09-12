package staking

import (
	"io"
	"math/big"

	"github.com/dfinlab/geth-zDollar/rlp"
	"github.com/dfinlab/meter/meter"
	"github.com/google/uuid"
)

var (
	CandidateMap map[meter.Address]*Candidate
)

// Candidate indicates the structure of a candidate
type Candidate struct {
	RewardAddr meter.Address // the address for staking / reward
	PubKey     []byte        // node public key
	IPAddr     []byte        // network addr
	Port       uint16
	Votes      *big.Int    // total voting from all buckets
	Buckets    []uuid.UUID // all buckets voted for this candidate
}

func NewCandidate(rewardAddr meter.Address, pubKey []byte, ip []byte, port uint16) *Candidate {
	return &Candidate{
		RewardAddr: rewardAddr,
		PubKey:     pubKey,
		IPAddr:     ip,
		Port:       port,
		Votes:      big.NewInt(0),
		Buckets:    []uuid.UUID{},
	}
}

func CandidateListToMap(candidateList []Candidate) error {
	for _, c := range candidateList {
		CandidateMap[c.RewardAddr] = &c
	}
	return nil
}

func CandidateMapToList() ([]Candidate, error) {
	candidateList := []Candidate{}
	for _, c := range CandidateMap {
		candidateList = append(candidateList, *c)
	}
	return candidateList, nil
}

// EncodeRLP implements rlp.Encoder.
func (c *Candidate) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		c.RewardAddr,
		c.PubKey,
		c.IPAddr,
		c.Port,
		c.Votes,
		c.Buckets,
	})

}

// DecodeRLP implements rlp.Decoder.
func (c *Candidate) DecodeRLP(stream *rlp.Stream) error {
	payload := struct {
		RewardAddr meter.Address
		PubKey     []byte
		IPAddr     []byte
		Port       uint16
		Votes      *big.Int
		Buckets    []uuid.UUID
	}{}

	if err := stream.Decode(&payload); err != nil {
		return err
	}

	*c = Candidate{
		RewardAddr: payload.RewardAddr,
		PubKey:     payload.PubKey,
		IPAddr:     payload.IPAddr,
		Port:       payload.Port,
		Votes:      payload.Votes,
		Buckets:    payload.Buckets,
	}
	return nil
}

func (c *Candidate) Add()    {}
func (c *Candidate) Update() {}
func (c *Candidate) Remove() {}
