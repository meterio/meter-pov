// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

// This is part of pacemaker that in charge of:
// 1. build outgoing messages
// 2. send messages to peer

import (
	"bytes"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
)

func (p *Pacemaker) sendMsg(msg ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.reactor.GetMyNetAddr()
	myName := p.reactor.GetMyName()
	myself := newConsensusPeer(myName, myNetAddr.IP, myNetAddr.Port, p.reactor.magic)

	round := msg.GetRound()

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *PMProposalMessage:
		peers = p.reactor.GetRelayPeers(round)
	case *PMVoteMessage:
		nxtProposer := p.getProposerByRound(round + 1)
		peers = append(peers, nxtProposer)
	case *PMTimeoutMessage:
		nxtProposer := p.getProposerByRound(round + 1)
		peers = append(peers, nxtProposer)
	}

	myselfInPeers := myself == nil
	for _, p := range peers {
		if p.netAddr.IP.String() == myNetAddr.IP.String() {
			myselfInPeers = true
			break
		}
	}
	// send consensus message to myself first (except for PMNewViewMessage)
	if copyMyself && !myselfInPeers {
		p.reactor.Send(msg, myself)
	}

	p.reactor.Send(msg, peers...)
	return true
}

func (p *Pacemaker) BuildProposalMessage(height, round uint32, bnew *draftBlock, tc *TimeoutCert) (*PMProposalMessage, error) {
	parentHeight := uint32(0)
	parentRound := uint32(0)
	if bnew.Parent != nil {
		parentHeight = bnew.Parent.Height
		parentRound = bnew.Parent.Round
	}
	msg := &PMProposalMessage{
		// Sender:    crypto.FromECDSAPub(&p.reactor.myPubKey),
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

		Height:       height,
		Round:        round,
		ParentHeight: parentHeight,
		ParentRound:  parentRound,

		RawBlock: bnew.RawBlock,

		TimeoutCert: tc,
	}

	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.reactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built Proposal Message", "height", msg.Height, "msg", msg.String(), "timestamp", msg.Timestamp)

	return msg, nil
}

// BuildVoteMsg build VFP message for proposal
// txRoot, stateRoot is decoded from proposalMsg.ProposedBlock, carry in cos already decoded outside
func (p *Pacemaker) BuildVoteMessage(proposalMsg *PMProposalMessage) (*PMVoteMessage, error) {

	proposedBlock := proposalMsg.DecodeBlock()
	voteHash := BuildBlockVotingHash(uint32(proposedBlock.BlockType()), uint64(proposalMsg.Height), proposedBlock.ID(), proposedBlock.TxsRoot(), proposedBlock.StateRoot())
	voteSig := p.reactor.blsCommon.SignHash(voteHash)
	// p.logger.Debug("Built PMVoteMessage", "signMsg", signMsg)

	msg := &PMVoteMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

		VoteHeight:    proposalMsg.Height,
		VoteRound:     proposalMsg.Round,
		VoteBlockID:   proposedBlock.ID(),
		VoteSignature: voteSig,
		VoteHash:      voteHash,
	}

	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.reactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built Vote Message", "msg", msg.String())
	return msg, nil
}

// BuildVoteForProposalMsg build VFP message for proposal
func (p *Pacemaker) BuildTimeoutMessage(qcHigh *draftQC, ti *PMRoundTimeoutInfo, lastVoteMsg *PMVoteMessage) (*PMTimeoutMessage, error) {

	// TODO: changed from nextHeight/nextRound to ti.height/ti.round, not sure if this is correct
	wishVoteHash := BuildTimeoutVotingHash(p.reactor.curEpoch, ti.round)
	wishVoteSig := p.reactor.blsCommon.SignHash(wishVoteHash)

	qcBytes, err := rlp.EncodeToBytes(qcHigh.QC)
	if err != nil {
		p.logger.Error("Error encode qc", "err", err)
	}
	msg := &PMTimeoutMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

		WishRound: ti.round + 1,

		QCHigh: qcBytes,

		WishVoteHash: wishVoteHash,
		WishVoteSig:  wishVoteSig,

		// LAST VOTE
	}

	if lastVoteMsg != nil {
		msg.LastVoteHeight = lastVoteMsg.VoteHeight
		msg.LastVoteRound = lastVoteMsg.VoteRound
		msg.LastVoteBlockID = lastVoteMsg.VoteBlockID
		msg.LastVoteHash = lastVoteMsg.VoteHash
		msg.LastVoteSignature = lastVoteMsg.VoteSignature
	}

	// if ti != nil {
	// 	msg.TimeoutHeight = nextHeight
	// 	msg.TimeoutRound = ti.round
	// 	msg.TimeoutCounter = ti.counter
	// }
	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.reactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built New View Message", "msg", msg.String())
	return msg, nil
}

// qc is for that block?
// blk is derived from draftBlock message. pass it in if already decoded
func draftBlockMatchQC(b *draftBlock, qc *block.QuorumCert) (bool, error) {

	if b == nil {
		// decode block to get qc
		// fmt.Println("can not decode block", err)
		return false, errors.New("can not decode proposed block")
	}

	// genesis does not have qc
	if b.Height == 0 && qc.QCHeight == 0 {
		return true, nil
	}

	blk := b.ProposedBlock

	return BlockMatchQC(blk, qc)
}

func BlockMatchQC(blk *block.Block, qc *block.QuorumCert) (bool, error) {

	voteHash := BuildBlockVotingHash(uint32(blk.BlockType()), uint64(blk.Number()), blk.ID(), blk.TxsRoot(), blk.StateRoot())
	//qc at least has 1 vote signature and they are the same, so compare [0] is good enough
	if bytes.Equal(voteHash[:], qc.VoterMsgHash[:]) {
		log.Debug("QC matches block", "qc", qc.String(), "block", blk.String())
		return true, nil
	} else {
		log.Warn("QC doesn't matches block", "msgHash", meter.Bytes32(voteHash).String(), "qc.VoteHash", meter.Bytes32(qc.VoterMsgHash).String())
		return false, nil
	}
}
