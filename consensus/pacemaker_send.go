// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

// This is part of pacemaker that in charge of:
// 1. build outgoing messages
// 2. send messages to peer

import (
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
)

func (p *Pacemaker) sendMsg(msg ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.reactor.GetMyNetAddr()
	myName := p.reactor.GetMyName()
	myself := newConsensusPeer(myName, myNetAddr.IP, myNetAddr.Port)

	round := msg.GetRound()

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *PMProposalMessage:
		peers = p.reactor.GetRelayPeers(round)
	case *PMVoteMessage:
		nxtProposer := p.getProposerByRound(round + 1)
		peers = append(peers, nxtProposer)
	case *PMTimeoutMessage:
		nxtProposer := p.getProposerByRound(round)
		peers = append(peers, nxtProposer)
	}

	myselfInPeers := myself == nil
	for _, p := range peers {
		if p.IP == myNetAddr.IP.String() {
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
	msg := &PMProposalMessage{
		// Sender:    crypto.FromECDSAPub(&p.reactor.myPubKey),
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

		Round:       round,
		RawBlock:    bnew.RawBlock,
		TimeoutCert: tc,
	}

	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.reactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built Proposal Message", "blk", bnew.ProposedBlock.ID().ToBlockShortID(), "msg", msg.String(), "timestamp", msg.Timestamp)

	return msg, nil
}

// BuildVoteMsg build VFP message for proposal
// txRoot, stateRoot is decoded from proposalMsg.ProposedBlock, carry in cos already decoded outside
func (p *Pacemaker) BuildVoteMessage(proposalMsg *PMProposalMessage) (*PMVoteMessage, error) {

	proposedBlock := proposalMsg.DecodeBlock()
	voteHash := proposedBlock.VotingHash()
	voteSig := p.reactor.blsCommon.SignHash(voteHash)
	// p.logger.Debug("Built PMVoteMessage", "signMsg", signMsg)

	msg := &PMVoteMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

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
		return nil, err
	}
	msg := &PMTimeoutMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

		WishRound: ti.round,

		QCHigh: qcBytes,

		WishVoteHash: wishVoteHash,
		WishVoteSig:  wishVoteSig,
	}

	// attach last vote
	if lastVoteMsg != nil {
		p.logger.Info("attached last vote", "votSig", hex.EncodeToString(lastVoteMsg.VoteSignature))
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
func BlockMatchDraftQC(b *draftBlock, escortQC *block.QuorumCert) bool {

	if b == nil {
		// decode block to get qc
		// fmt.Println("can not decode block", err)
		return false
	}

	// genesis does not have qc
	if b.Height == 0 && escortQC.QCHeight == 0 {
		return true
	}

	blk := b.ProposedBlock

	return blk.MatchQC(escortQC)
}

// BuildQueryMessage
func (p *Pacemaker) BuildQueryMessage() (*PMQueryMessage, error) {
	msg := &PMQueryMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.GetMyActualCommitteeIndex()),

		LastCommitted: p.lastCommitted.ProposedBlock.ID(),
	}

	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.reactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built Query Message", "msg", msg.String())
	return msg, nil
}
