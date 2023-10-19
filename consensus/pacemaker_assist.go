// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/meterio/meter-pov/block"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/types"
)

// This is part of pacemaker that in charge of:
// 1. pending proposal/newView
// 2. timeout cert management

// check a DraftBlock is the extension of b_locked, max 10 hops
func (p *Pacemaker) ExtendedFromLastCommitted(b *block.DraftBlock) bool {

	i := int(0)
	tmp := b
	for i < 10 {
		if tmp == p.lastCommitted {
			return true
		}
		if tmp = tmp.Parent; tmp == nil {
			break
		}
		i++
	}
	return false
}

func (p *Pacemaker) ValidateProposal(b *block.DraftBlock) error {
	start := time.Now()
	blk := b.ProposedBlock

	// special valiadte StopCommitteeType
	// possible 2 rounds of stop messagB
	if blk.IsSBlock() {
		parent := p.chain.GetDraft(b.ProposedBlock.ParentID())
		if !parent.ProposedBlock.IsKBlock() && !parent.ProposedBlock.IsSBlock() {
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

	pool := p.reactor.txpool
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}

	var txsInBlk []*tx.Transaction
	for _, tx := range blk.Transactions() {
		txsInBlk = append(txsInBlk, tx)
	}
	var txsToRemoved, returnTxsToPool func()
	if b.ProposedBlock.IsKBlock() {
		txsToRemoved = func() {}
		returnTxsToPool = func() {}
	} else {
		txsToRemoved = func() {
			for _, tx := range txsInBlk {
				pool.Remove(tx.ID())
			}
		}
		returnTxsToPool = func() {
			for _, tx := range txsInBlk {
				// only return txs if they are not in database
				meta, err := p.chain.GetTrunkTransactionMeta(tx.ID())
				if meta == nil || err != nil {
					pool.Add(tx)
				}
			}
		}
	}

	checkpointStart := time.Now()
	//create checkPoint before validate block
	state, err := p.reactor.stateCreator.NewState(p.reactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "height", blk.Number(), "id", blk.ID(), "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()
	checkpointElapsed := time.Since(checkpointStart)

	// make sure tx does not exist in draft cache
	if len(blk.Transactions()) > 0 {
		txsInCache := make(map[string]bool)
		tmp := parent
		for tmp != nil && !tmp.Committed {
			for _, knownTx := range tmp.ProposedBlock.Transactions() {
				txsInCache[knownTx.ID().String()] = true
			}
			tmp = p.chain.GetDraft(tmp.ProposedBlock.ParentID())
		}
		for _, tx := range blk.Transactions() {
			if _, existed := txsInCache[tx.ID().String()]; existed {
				p.logger.Error("tx already existed in cache", "id", tx.ID(), "containedInBlock", parent.ProposedBlock.ID())
				return errors.New("tx already existed in cache")

			}
		}
	}

	processStart := time.Now()
	now := uint64(time.Now().Unix())
	stage, receipts, err := p.reactor.ProcessProposedBlock(parentBlock, blk, now)
	if err != nil && err != errKnownBlock {
		p.logger.Error("process proposed failed", "proposed", blk.Oneliner(), "err", err)
		b.SuccessProcessed = false
		b.ProcessError = err
		return err
	}
	processElapsed := time.Since(processStart)

	stageCommitStart := time.Now()
	if stage == nil {
		// FIXME: probably should not handle this proposal any more
		p.logger.Warn("Empty stage !!!")
	} else if _, err := stage.Commit(); err != nil {
		p.logger.Warn("commit stage failed: ", "err", err)
		b.SuccessProcessed = false
		b.ProcessError = err
		return err
	}
	stageCommitElapsed := time.Since(stageCommitStart)

	// p.logger.Info(fmt.Sprintf("cached %s", blk.ID().ToBlockShortID()))

	b.Stage = stage
	b.Receipts = &receipts
	b.CheckPoint = checkPoint
	b.ReturnTxsToPool = returnTxsToPool
	b.SuccessProcessed = true
	b.ProcessError = err

	txRemoveStart := time.Now()
	txsToRemoved()
	txRemoveElapsed := time.Since(txRemoveStart)

	p.logger.Info(fmt.Sprintf("validated proposal %s R:%v, %v, txs:%d", b.ProposedBlock.GetCanonicalName(), b.Round, blk.ShortID(), len(b.ProposedBlock.Transactions())), "elapsed", meter.PrettyDuration(time.Since(start)), "checkpointElapsed", meter.PrettyDuration(checkpointElapsed), "processElapsed", meter.PrettyDuration(processElapsed), "stageCommitElapsed", meter.PrettyDuration(stageCommitElapsed), "txRemoveElapsed", meter.PrettyDuration(txRemoveElapsed))
	return nil
}

func (p *Pacemaker) getProposerByRound(round uint32) *ConsensusPeer {
	proposer := p.reactor.getRoundProposer(round)
	return newConsensusPeer(proposer.Name, proposer.NetAddr.IP, 8670)
}

func (p *Pacemaker) verifyTC(tc *types.TimeoutCert, round uint32) bool {
	if tc != nil {
		voteHash := BuildTimeoutVotingHash(tc.Epoch, tc.Round)
		pubkeys := make([]bls.PublicKey, 0)

		// check epoch and round
		if tc.Epoch != p.reactor.curEpoch || tc.Round != round {
			return false

		}
		// check hash
		if !bytes.Equal(tc.MsgHash[:], voteHash[:]) {
			return false
		}
		// check vote count
		voteCount := tc.BitArray.Count()
		if !block.MajorityTwoThird(uint32(voteCount), p.reactor.committeeSize) {
			return false
		}

		// check signature
		for index, v := range p.reactor.committee {
			if tc.BitArray.GetIndex(index) {
				pubkeys = append(pubkeys, v.BlsPubKey)
			}
		}
		sig, err := p.reactor.blsCommon.System.SigFromBytes(tc.AggSig)
		if err != nil {
			return false
		}
		valid, err := p.reactor.blsCommon.AggregateVerify(sig, tc.MsgHash, pubkeys)
		if err != nil {
			return false
		}
		if !valid {
			p.logger.Warn("Invalid TC", "expected", fmt.Sprintf("E:%v,R:%v", tc.Epoch, tc.Round), "proposal", fmt.Sprintf("E:%v,R:%v", p.reactor.curEpoch, round))
		}
		return valid
	}
	return false
}
