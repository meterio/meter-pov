// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"errors"
	"fmt"
	"time"

	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
)

// This is part of pacemaker that in charge of:
// 1. pending proposal/newView
// 2. timeout cert management

// check a draftBlock is the extension of b_locked, max 10 hops
func (p *Pacemaker) IsExtendedFromBLocked(b *draftBlock) bool {

	i := int(0)
	tmp := b
	for i < 10 {
		if tmp == p.blockLocked {
			return true
		}
		if tmp = tmp.Parent; tmp == nil {
			break
		}
		i++
	}
	return false
}

func (p *Pacemaker) ValidateProposal(b *draftBlock) error {
	blk := b.ProposedBlock

	// special valiadte StopCommitteeType
	// possible 2 rounds of stop messagB
	if BlockType(blk.BlockType()) == StopCommitteeType {
		parent := p.proposalMap.GetByID(b.ProposedBlock.ParentID())
		if parent.BlockType != KBlockType && parent.BlockType != StopCommitteeType {
			p.logger.Info(fmt.Sprintf("proposal [%d] is the first stop committee block", b.Height))
			return errors.New("sBlock should have kBlock/sBlock parent")
		}
	}

	// avoid duplicate validation
	if b.SuccessProcessed && b.ProcessError == nil {
		return nil
	}

	parent := b.Parent
	if parent == nil || parent.ProposedBlock == nil {
		return errParentMissing
	}
	parentBlock := parent.ProposedBlock
	if parentBlock == nil {
		return errDecodeParentFailed
	}
	parentHeader := parentBlock.Header()

	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}

	var txsInBlk []*tx.Transaction
	for _, tx := range blk.Transactions() {
		txsInBlk = append(txsInBlk, tx)
	}
	var txsToRemoved, txsToReturned func() bool
	if b.BlockType == KBlockType {
		txsToRemoved = func() bool { return true }
		txsToReturned = func() bool { return true }
	} else {
		txsToRemoved = func() bool {
			for _, tx := range txsInBlk {
				pool.Remove(tx.ID())
			}
			return true
		}
		txsToReturned = func() bool {
			for _, tx := range txsInBlk {
				pool.Add(tx)
			}
			return true
		}
	}

	//create checkPoint before validate block
	state, err := p.reactor.stateCreator.NewState(p.reactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "height", blk.Number(), "id", blk.ID(), "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

	now := uint64(time.Now().Unix())
	stage, receipts, err := p.reactor.ProcessProposedBlock(parentHeader, blk, now)
	if err != nil && err != errKnownBlock {
		p.logger.Error("process block failed", "proposed", blk.Oneliner(), "err", err)
		b.SuccessProcessed = false
		b.ProcessError = err
		return err
	}

	b.Stage = stage
	b.Receipts = &receipts
	b.CheckPoint = checkPoint
	b.txsToRemoved = txsToRemoved
	b.txsToReturned = txsToReturned
	b.SuccessProcessed = true
	b.ProcessError = err

	p.logger.Info(fmt.Sprintf("validated proposal %v", blk.ShortID()))
	return nil
}

func (p *Pacemaker) getProposerByRound(round uint32) *ConsensusPeer {
	proposer := p.reactor.getRoundProposer(round)
	return newConsensusPeer(proposer.Name, proposer.NetAddr.IP, 8080, p.reactor.magic)
}

func (p *Pacemaker) verifyTimeoutCert(tc *TimeoutCert, round uint32) bool {
	if tc != nil {
		//FIXME: check timeout cert
		valid := tc.Epoch == p.reactor.curEpoch && tc.Round == round
		if !valid {
			p.logger.Warn("Invalid TC", "expected", fmt.Sprintf("E:%v,R:%v", tc.Epoch, tc.Round), "proposal", fmt.Sprintf("E:%v,R:%v", p.reactor.curEpoch, round))
		}
		return valid
	}
	return false
}
