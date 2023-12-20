// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

// This is part of pacemaker that in charge of:
// 1. build outgoing messages
// 2. send messages to peer

import (
	sha256 "crypto/sha256"
	"fmt"
	"time"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/types"
)

func (p *Pacemaker) sendMsg(msg block.ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.reactor.GetMyNetAddr()
	myName := p.reactor.GetMyName()
	myself := NewConsensusPeer(myName, myNetAddr.IP.String())

	round := msg.GetRound()

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *block.PMProposalMessage:
		peers = p.reactor.GetRelayPeers(round)
	case *block.PMVoteMessage:
		nxtProposer := p.getProposerByRound(round + 1)
		peers = append(peers, nxtProposer)
	case *block.PMTimeoutMessage:
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

func (p *Pacemaker) BuildProposalMessage(height, round uint32, bnew *block.DraftBlock, tc *types.TimeoutCert) (*block.PMProposalMessage, error) {
	raw, err := rlp.EncodeToBytes(bnew.ProposedBlock)
	if err != nil {
		return nil, err
	}
	msg := &block.PMProposalMessage{
		// Sender:    crypto.FromECDSAPub(&p.reactor.myPubKey),
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.committeeIndex),

		Round:       round,
		RawBlock:    raw,
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
func (p *Pacemaker) BuildVoteMessage(proposalMsg *block.PMProposalMessage) (*block.PMVoteMessage, error) {

	proposedBlock := proposalMsg.DecodeBlock()
	voteHash := proposedBlock.VotingHash()
	voteSig := p.reactor.blsCommon.SignHash(voteHash)
	// p.logger.Debug("Built PMVoteMessage", "signMsg", signMsg)

	msg := &block.PMVoteMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.committeeIndex),

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

// Timeout Vote Message Hash
func BuildTimeoutVotingHash(epoch uint64, round uint32) [32]byte {
	msg := fmt.Sprintf("Timeout: Epoch:%v Round:%v", epoch, round)
	voteHash := sha256.Sum256([]byte(msg))
	return voteHash
}

// BuildVoteForProposalMsg build VFP message for proposal
func (p *Pacemaker) BuildTimeoutMessage(qcHigh *block.DraftQC, ti *PMRoundTimeoutInfo, lastVoteMsg *block.PMVoteMessage) (*block.PMTimeoutMessage, error) {

	// TODO: changed from nextHeight/nextRound to ti.height/ti.round, not sure if this is correct
	wishVoteHash := BuildTimeoutVotingHash(p.reactor.curEpoch, ti.round)
	wishVoteSig := p.reactor.blsCommon.SignHash(wishVoteHash)

	rawQC, err := rlp.EncodeToBytes(qcHigh.QC)
	if err != nil {
		return nil, err
	}
	msg := &block.PMTimeoutMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.committeeIndex),

		WishRound: ti.round,

		QCHigh: rawQC,

		WishVoteHash: wishVoteHash,
		WishVoteSig:  wishVoteSig,
	}

	// attach last vote
	if lastVoteMsg != nil {
		p.logger.Info(fmt.Sprintf("attached last vote on R:%d", lastVoteMsg.VoteRound), "blk", lastVoteMsg.VoteBlockID.ToBlockShortID())
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

// BuildQueryMessage
func (p *Pacemaker) BuildQueryMessage() (*block.PMQueryMessage, error) {
	msg := &block.PMQueryMessage{
		Timestamp:   time.Now(),
		Epoch:       p.reactor.curEpoch,
		SignerIndex: uint32(p.reactor.committeeIndex),

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
