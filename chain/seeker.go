// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain

import (
	"fmt"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
)

// Seeker to seek block by given number on the chain defined by head block ID.
type Seeker struct {
	chain       *Chain
	headBlockID meter.Bytes32
	err         error
}

func newSeeker(chain *Chain, headBlockID meter.Bytes32) *Seeker {
	return &Seeker{
		chain:       chain,
		headBlockID: headBlockID,
	}
}

func (s *Seeker) setError(err error) {
	if s.err == nil {
		s.err = err
	}
}

// Err returns error occurred.
func (s *Seeker) Err() error {
	return s.err
}

// GetID returns block ID by the given number.
func (s *Seeker) GetID(num uint32) meter.Bytes32 {
	if num > block.Number(s.headBlockID) {
		panic("num exceeds head block")
	}

	// query draft space
	draft := s.chain.GetDraftByNum(num)
	if draft != nil {
		return draft.ProposedBlock.ID()
	}

	id, err := s.chain.GetAncestorBlockID(s.headBlockID, num)
	if err != nil {
		fmt.Println("GetAncestorBlockID error in seeker.GetID", "headBlockID", s.headBlockID, "num", num, "err", err)
		panic(err)
	}
	s.setError(err)
	return id
}

// GetHeader returns block header by the given number.
func (s *Seeker) GetHeader(id meter.Bytes32) *block.Header {
	header, err := s.chain.GetBlockHeader(id)
	if err != nil {
		fmt.Println("chain.GetBlockHeaer error in seeker.GetHeader", "id", id, "err", err)
		s.setError(err)
		return &block.Header{}
	}
	return header
}

// GenesisID get genesis block ID.
func (s *Seeker) GenesisID() meter.Bytes32 {
	return s.chain.GenesisBlock().ID()
}
