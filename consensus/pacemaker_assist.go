// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"errors"
	"fmt"
	"time"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
)

// This is part of pacemaker that in charge of:
// 1. pending proposal/newView
// 2. timeout cert management

const (
	MSG_KEEP_HEIGHT = 80
)

type BlockProbe struct {
	Height uint32
	Round  uint32
	Type   uint32
	Raw    []byte
}

// check a pmBlock is the extension of b_locked, max 10 hops
func (p *Pacemaker) IsExtendedFromBLocked(b *pmBlock) bool {

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

// find out b b' b"
func (p *Pacemaker) AddressBlock(height uint32) *pmBlock {
	blk := p.proposalMap.Get(height)
	if blk != nil && blk.Height == height {
		//p.logger.Debug("Addressed block", "height", height, "round", round)
		return blk
	}

	p.logger.Debug("can not address block", "height", height)
	return nil
}

func (p *Pacemaker) ValidateProposal(b *pmBlock) error {
	blockBytes := b.ProposedBlock
	blk, err := block.BlockDecodeFromBytes(blockBytes)
	if err != nil {
		p.logger.Error("Decode block failed", "err", err)
		return err
	}

	// special valiadte StopCommitteeType
	// possible 2 rounds of stop messagB
	if b.ProposedBlockType == StopCommitteeType {

		parent := p.proposalMap.Get(b.Height - 1)
		if parent.ProposedBlockType == KBlockType {
			p.logger.Info(fmt.Sprintf("proposal [%d] is the first stop committee block", b.Height))
		} else if parent.ProposedBlockType == StopCommitteeType {
			grandParent := p.proposalMap.Get(b.Height - 2)
			if grandParent.ProposedBlockType == KBlockType {
				p.logger.Info(fmt.Sprintf("proposal [%d] is the second stop committee block", b.Height))

			}
		}
	}

	/*
		if b.ProposedBlockInfo != nil {
			// if this proposal is proposed by myself, don't execute it again
			p.logger.Debug("this proposal is created by myself, skip the validation...")
			b.SuccessProcessed = true
			return nil
		}
	*/

	parentPMBlock := b.Parent
	if parentPMBlock == nil || parentPMBlock.ProposedBlock == nil {
		return errParentMissing
	}
	parentBlock, err := block.BlockDecodeFromBytes(parentPMBlock.ProposedBlock)
	if err != nil {
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
	if b.ProposedBlockType == KBlockType {
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
	state, err := p.csReactor.stateCreator.NewState(p.csReactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "height", blk.Number(), "id", blk.ID(), "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

	now := uint64(time.Now().Unix())
	stage, receipts, err := p.csReactor.ProcessProposedBlock(parentHeader, blk, now)
	if err != nil && err != errKnownBlock {
		p.logger.Error("process block failed", "proposed", blk.Oneliner(), "err", err)
		b.SuccessProcessed = false
		b.ProcessError = err
		return err
	}

	b.ProposedBlockInfo = &ProposedBlockInfo{
		BlockType:     b.ProposedBlockType,
		ProposedBlock: blk,
		Stage:         stage,
		Receipts:      &receipts,
		CheckPoint:    checkPoint,
		txsToRemoved:  txsToRemoved,
		txsToReturned: txsToReturned,
	}

	b.SuccessProcessed = true
	b.ProcessError = err

	p.logger.Info(fmt.Sprintf("validated proposal %v", blk.ShortID()))
	return nil
}

func (p *Pacemaker) getProposerByRound(round uint32) *ConsensusPeer {
	proposer := p.csReactor.getRoundProposer(round)
	return newConsensusPeer(proposer.Name, proposer.NetAddr.IP, 8080, p.csReactor.magic)
}

func (p *Pacemaker) verifyTimeoutCert(tc *PMTimeoutCert, height, round uint32) bool {
	if tc != nil {
		//FIXME: check timeout cert
		valid := tc.TimeoutHeight <= height && tc.TimeoutRound == round
		if !valid {
			p.logger.Warn("Invalid Timeout Cert", "expected", fmt.Sprintf("H:%v,R:%v", tc.TimeoutHeight, tc.TimeoutRound), "proposal", fmt.Sprintf("H:%v,R:%v", height, round))
		}
		return valid
	}
	return false
}

// for proposals which can not be addressed parent and QC node should
// put it to pending list and query the parent node
func (p *Pacemaker) sendQueryProposalMsg(fromHeight, toHeight, queryRound uint32, EpochID uint64, peer *ConsensusPeer) error {
	if fromHeight > toHeight && toHeight != 0 {
		p.logger.Info(fmt.Sprintf("skip query %v->%v, not necessary ", fromHeight, toHeight))
		return nil
	}
	key := QueryKey{From: fromHeight, To: toHeight}
	if count, exist := p.queryCount[key]; !exist {
		p.queryCount[key] = 1
	} else {
		p.queryCount[key] = count + 1
	}
	if p.queryCount[key] > 3 {
		p.logger.Info(fmt.Sprintf("skip query %v->%v, already %v queries sent", fromHeight, toHeight, p.queryCount[key]))
		return nil
	}
	// put this proposal to pending list, and sent out query
	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr

	// sometimes we find out addr is my self, protection is added here
	if myNetAddr.IP.Equal(peer.netAddr.IP) == true {
		for _, cm := range p.csReactor.curActualCommittee {
			if myNetAddr.IP.Equal(cm.NetAddr.IP) == false {
				p.logger.Warn("Query PMProposal with new node", "NetAddr", cm.NetAddr)
				peer = newConsensusPeer(cm.Name, cm.NetAddr.IP, cm.NetAddr.Port, p.csReactor.magic)
				break
			}
		}
	}

	// fromHeight must be less than or equal to toHeight, except when toHeight is 0
	// in this case, 0 is interpreted as infinity (or local qcHigh)
	if fromHeight > toHeight && toHeight != 0 {
		p.logger.Info("query not necessary", "fromHeight", fromHeight, "toHeight", toHeight)
		return nil
	}

	queryMsg, err := p.BuildQueryProposalMessage(fromHeight, toHeight, queryRound, EpochID, myNetAddr)
	if err != nil {
		p.logger.Warn("failed to generate PMQueryProposal message", "err", err)
		return errors.New("failed to generate PMQueryProposal message")
	}
	p.sendMsgToPeer(queryMsg, false, peer)
	return nil
}

func (p *Pacemaker) pendingProposal(queryHeight, queryRound uint32, epochID uint64, mi *consensusMsgInfo) error {
	fromHeight := p.lastVotingHeight
	if p.QCHigh != nil && p.QCHigh.QCNode != nil && fromHeight < p.QCHigh.QCNode.Height {
		fromHeight = p.QCHigh.QCNode.Height
	}
	if err := p.sendQueryProposalMsg(fromHeight, queryHeight, queryRound, epochID, mi.Peer); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(mi)
	return nil
}

// put it to pending list and query the parent node
func (p *Pacemaker) pendingNewView(queryHeight, queryRound uint32, epochID uint64, mi *consensusMsgInfo) error {
	bestQC := p.csReactor.chain.BestQC()
	fromHeight := p.lastVotingHeight
	if p.QCHigh != nil && p.QCHigh.QCNode != nil && fromHeight < p.QCHigh.QCNode.Height {
		fromHeight = bestQC.QCHeight
	}
	if err := p.sendQueryProposalMsg(fromHeight, queryHeight, queryRound, epochID, mi.Peer); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(mi)
	return nil
}

func (p *Pacemaker) checkPendingMessages(curHeight uint32) error {
	height := curHeight
	count := 0
	if pendingMsg, ok := p.pendingList.messages[height]; ok {
		count++
		// height++ //move higher
		capacity := cap(p.pacemakerMsgCh)
		msgs := make([]consensusMsgInfo, 0)
		for len(p.pacemakerMsgCh) > 0 {
			msgs = append(msgs, <-p.pacemakerMsgCh)
		}

		// promote pending msg to the very next
		p.pacemakerMsgCh <- pendingMsg

		for _, msg := range msgs {
			if len(p.pacemakerMsgCh) < capacity {
				p.pacemakerMsgCh <- msg
			}
		}
	}
	if count > 0 {
		p.logger.Info("Found pending message", "height", height)
	}

	lowest := p.pendingList.GetLowestHeight()
	if (height > lowest) && (height-lowest) >= 3*MSG_KEEP_HEIGHT {
		p.pendingList.CleanUpTo(height - MSG_KEEP_HEIGHT)
	}
	return nil
}
