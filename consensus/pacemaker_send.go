// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

// This is part of pacemaker that in charge of:
// 1. build outgoing messages
// 2. send messages to peer

import (
	"bytes"
	sha256 "crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/meter"
)

func (p *Pacemaker) sendMsg(msg ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.csReactor.GetMyNetAddr()
	myName := p.csReactor.GetMyName()
	myself := newConsensusPeer(myName, myNetAddr.IP, myNetAddr.Port, p.csReactor.magic)

	round := msg.GetRound()

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *PMProposalMessage:
		peers = p.csReactor.GetRelayPeers(round)
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
	typeName := msg.GetType()
	if copyMyself && !myselfInPeers {
		p.logger.Debug(fmt.Sprintf("Sending %v to myself", typeName))
		p.sendMsgToPeer(msg, false, myself)
	}

	peerNames := make([]string, 0)
	for _, p := range peers {
		peerNames = append(peerNames, p.name)
	}
	p.logger.Debug(fmt.Sprintf("Sending %v to peers: %v", typeName, strings.Join(peerNames, ",")))
	p.sendMsgToPeer(msg, false, peers...)
	return true
}

func (p *Pacemaker) sendMsgToPeer(msg ConsensusMessage, relay bool, peers ...*ConsensusPeer) bool {
	data, err := p.csReactor.MarshalMsg(&msg)
	if err != nil {
		fmt.Println("error marshaling message", err)
		return false
	}
	msgSummary := msg.String()
	msgHash := sha256.Sum256(data)
	msgHashHex := hex.EncodeToString(msgHash[:])[:8]

	peerNames := make([]string, 0)
	for _, peer := range peers {
		peerNames = append(peerNames, peer.NameAndIP())
	}
	prefix := "send"
	if relay {
		prefix = "relay"
	}
	p.logger.Info(prefix+" "+msgSummary, "to", strings.Join(peerNames, ", "))
	// broadcast consensus message to peers
	for _, peer := range peers {
		go peer.sendPacemakerMsg(data, msgSummary, msgHashHex, relay)
	}
	return true
}

func (p *Pacemaker) BuildProposalMessage(height, round uint32, bnew *pmBlock, tc *TimeoutCert) (*PMProposalMessage, error) {
	parentHeight := uint32(0)
	parentRound := uint32(0)
	if bnew.Parent != nil {
		parentHeight = bnew.Parent.Height
		parentRound = bnew.Parent.Round
	}
	msg := &PMProposalMessage{
		// Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp:   time.Now(),
		Epoch:       p.csReactor.curEpoch,
		SignerIndex: uint32(p.myActualCommitteeIndex),

		Height:       height,
		Round:        round,
		ParentHeight: parentHeight,
		ParentRound:  parentRound,

		RawBlock: bnew.RawBlock,

		TimeoutCert: tc,
	}

	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.csReactor.myPrivKey)
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
	voteSig := p.csReactor.csCommon.SignHash(voteHash)
	// p.logger.Debug("Built PMVoteMessage", "signMsg", signMsg)

	msg := &PMVoteMessage{
		Timestamp:   time.Now(),
		Epoch:       p.csReactor.curEpoch,
		SignerIndex: uint32(p.myActualCommitteeIndex),

		VoteHeight:    proposalMsg.Height,
		VoteRound:     proposalMsg.Round,
		VoteBlockID:   proposedBlock.ID(),
		VoteSignature: voteSig,
		VoteHash:      voteHash,
	}

	// sign message
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.csReactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built Vote Message", "msg", msg.String())
	return msg, nil
}

// BuildVoteForProposalMsg build VFP message for proposal
func (p *Pacemaker) BuildTimeoutMessage(qcHigh *pmQuorumCert, ti *PMRoundTimeoutInfo, lastVoteMsg *PMVoteMessage) (*PMTimeoutMessage, error) {

	// TODO: changed from nextHeight/nextRound to ti.height/ti.round, not sure if this is correct
	wishVoteHash := BuildTimeoutVotingHash(p.csReactor.curEpoch, ti.round)
	wishVoteSig := p.csReactor.csCommon.SignHash(wishVoteHash)

	qcBytes, err := rlp.EncodeToBytes(qcHigh.QC)
	if err != nil {
		p.logger.Error("Error encode qc", "err", err)
	}
	msg := &PMTimeoutMessage{
		Timestamp:   time.Now(),
		Epoch:       p.csReactor.curEpoch,
		SignerIndex: uint32(p.myActualCommitteeIndex),

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
	msgSig, err := crypto.Sign(msg.GetMsgHash().Bytes(), &p.csReactor.myPrivKey)
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.SetMsgSignature(msgSig)
	p.logger.Debug("Built New View Message", "msg", msg.String())
	return msg, nil
}

// qc is for that block?
// blk is derived from pmBlock message. pass it in if already decoded
func BlockMatchQC(b *pmBlock, qc *block.QuorumCert) (bool, error) {

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

	voteHash := BuildBlockVotingHash(uint32(b.BlockType), uint64(b.Height), blk.ID(), blk.TxsRoot(), blk.StateRoot())
	//qc at least has 1 vote signature and they are the same, so compare [0] is good enough
	if bytes.Equal(voteHash[:], qc.VoterMsgHash[:]) {
		log.Debug("QC matches block", "qc", qc.String(), "block", blk.String())
		return true, nil
	} else {
		log.Warn("QC doesn't matches block", "msgHash", meter.Bytes32(voteHash).String(), "qc.VoteHash", meter.Bytes32(qc.VoterMsgHash).String())
		return false, nil
	}
}
