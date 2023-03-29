// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

// This is part of pacemaker that in charge of:
// 1. build outgoing messages
// 2. send messages to peer

import (
	"bytes"
	"crypto/ecdsa"
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
	"github.com/meterio/meter-pov/types"
)

func (p *Pacemaker) sendMsg(round uint32, msg ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.csReactor.GetMyNetAddr()
	myName := p.csReactor.GetMyName()
	myself := newConsensusPeer(myName, myNetAddr.IP, myNetAddr.Port, p.csReactor.magic)

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *PMProposalMessage:
		peers = p.GetRelayPeers(round)
	case *PMVoteMessage:
		proposer := p.getProposerByRound(round)
		peers = append(peers, proposer)
	case *PMNewViewMessage:
		nv := msg.(*PMNewViewMessage)
		if nv.Reason == HigherQCSeen {
			visited := make(map[string]bool)
			for i := 0; i < meter.NewViewPeersLimit; i++ {
				nxtProposer := p.getProposerByRound(round + uint32(i))
				if _, ok := visited[nxtProposer.String()]; !ok {
					peers = append(peers, nxtProposer)
					visited[nxtProposer.String()] = true
				}
			}
		} else {
			nxtProposer := p.getProposerByRound(round)
			peers = append(peers, nxtProposer)
		}
	}

	myselfInPeers := myself == nil
	for _, p := range peers {
		if p.netAddr.IP.String() == myNetAddr.IP.String() {
			myselfInPeers = true
			break
		}
	}
	// send consensus message to myself first (except for PMNewViewMessage)
	typeName := getConcreteName(msg)
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
		peerNames = append(peerNames, peer.NameString())
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

func (p *Pacemaker) BuildProposalMessage(height, round uint32, bnew *pmBlock, tc *PMTimeoutCert) (*PMProposalMessage, error) {
	blockBytes := bnew.ProposedBlock

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    height,
		Round:     round,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   PACEMAKER_MSG_PROPOSAL,

		// MsgSubType: msgSubType,
		EpochID: p.csReactor.curEpoch,
	}

	parentHeight := uint32(0)
	parentRound := uint32(0)
	if bnew.Parent != nil {
		parentHeight = bnew.Parent.Height
		parentRound = bnew.Parent.Round
	}
	msg := &PMProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		ParentHeight: parentHeight,
		ParentRound:  parentRound,

		ProposerID:        crypto.FromECDSAPub(&p.csReactor.myPubKey),
		ProposerBlsPK:     p.csReactor.csCommon.GetSystem().PubKeyToBytes(*p.csReactor.csCommon.GetPublicKey()),
		KBlockHeight:      p.csReactor.lastKBlockHeight,
		ProposedSize:      uint32(len(blockBytes)),
		ProposedBlock:     blockBytes,
		ProposedBlockType: bnew.ProposedBlockType,

		TimeoutCert: tc,
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Built Proposal Message", "height", msg.CSMsgCommonHeader.Height, "msg", msg.String(), "timestamp", msg.CSMsgCommonHeader.Timestamp)

	return msg, nil
}

// BuildVoteForProposalMsg build VFP message for proposal
// txRoot, stateRoot is decoded from proposalMsg.ProposedBlock, carry in cos already decoded outside
func (p *Pacemaker) BuildVoteForProposalMessage(proposalMsg *PMProposalMessage, blockID, txsRoot, stateRoot meter.Bytes32) (*PMVoteMessage, error) {

	ch := proposalMsg.CSMsgCommonHeader

	signMsg := p.csReactor.BuildProposalBlockSignMsg(uint32(proposalMsg.ProposedBlockType), uint64(ch.Height), &blockID, &txsRoot, &stateRoot)
	sign, msgHash := p.csReactor.csCommon.SignMessage([]byte(signMsg))
	p.logger.Debug("Built PMVoteMessage", "signMsg", signMsg)

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    ch.Height,
		Round:     ch.Round,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   PACEMAKER_MSG_VOTE,

		EpochID: p.csReactor.curEpoch,
	}

	index := p.csReactor.GetCommitteeMemberIndex(p.csReactor.myPubKey)
	msg := &PMVoteMessage{
		CSMsgCommonHeader: cmnHdr,

		VoterID:           crypto.FromECDSAPub(&p.csReactor.myPubKey),
		VoterBlsPK:        p.csReactor.csCommon.GetSystem().PubKeyToBytes(*p.csReactor.csCommon.GetPublicKey()),
		BlsSignature:      p.csReactor.csCommon.GetSystem().SigToBytes(sign),
		VoterIndex:        uint32(index),
		SignedMessageHash: msgHash,
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Built Vote For Proposal Message", "msg", msg.String())
	return msg, nil
}

// BuildVoteForProposalMsg build VFP message for proposal
func (p *Pacemaker) BuildNewViewMessage(nextHeight, nextRound uint32, qcHigh *pmQuorumCert, reason NewViewReason, ti *PMRoundTimeoutInfo) (*PMNewViewMessage, error) {

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    nextHeight,
		Round:     nextRound,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   PACEMAKER_MSG_NEW_VIEW,

		EpochID: p.csReactor.curEpoch,
	}

	index := p.csReactor.GetCommitteeMemberIndex(p.csReactor.myPubKey)

	signMsg := p.BuildNewViewSignMsg(p.csReactor.myPubKey, reason, nextHeight, nextRound, qcHigh.QC)

	sign, msgHash := p.csReactor.csCommon.SignMessage([]byte(signMsg))

	qcBytes, err := rlp.EncodeToBytes(qcHigh.QC)
	if err != nil {
		p.logger.Error("Error encode qc", "err", err)
	}
	msg := &PMNewViewMessage{
		CSMsgCommonHeader: cmnHdr,

		QCHeight: qcHigh.QC.QCHeight,
		QCRound:  qcHigh.QC.QCRound,
		QCHigh:   qcBytes,
		Reason:   reason,

		PeerID:            crypto.FromECDSAPub(&p.csReactor.myPubKey),
		PeerIndex:         uint32(index),
		SignedMessageHash: msgHash,
		PeerSignature:     p.csReactor.csCommon.GetSystem().SigToBytes(sign),
	}

	if ti != nil {
		msg.TimeoutHeight = nextHeight
		msg.TimeoutRound = ti.round
		msg.TimeoutCounter = ti.counter
	}
	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Built New View Message", "msg", msg.String())
	return msg, nil
}

func (p *Pacemaker) BuildNewViewSignMsg(pubKey ecdsa.PublicKey, reason NewViewReason, height, round uint32, qc *block.QuorumCert) string {
	return fmt.Sprintf("New View Message: Peer:%s Height:%v Round:%v Reason:%v QC:(%d,%d,%v,%v)",
		hex.EncodeToString(crypto.FromECDSAPub(&pubKey)), height, round, reason, qc.QCHeight, qc.QCRound, qc.EpochID, hex.EncodeToString(qc.VoterAggSig))
}

func (p *Pacemaker) BuildQueryProposalMessage(fromHeight, toHeight, round uint32, epochID uint64, retAddr types.NetAddress) (*PMQueryProposalMessage, error) {
	cmnHdr := ConsensusMsgCommonHeader{
		Height:    0,
		Round:     0,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   PACEMAKER_MSG_QUERY_PROPOSAL,

		// MsgSubType: msgSubType,
		EpochID: epochID,
	}

	msg := &PMQueryProposalMessage{
		CSMsgCommonHeader: cmnHdr,
		FromHeight:        fromHeight,
		ToHeight:          toHeight,
		Round:             round,
		ReturnAddr:        retAddr,
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	// p.logger.Debug("Built Query Proposal Message", "height", height, "round", round, "msg", msg.String(), "timestamp", msg.CSMsgCommonHeader.Timestamp)

	return msg, nil
}

// qc is for that block?
// blk is derived from pmBlock message. pass it in if already decoded
func (p *Pacemaker) BlockMatchQC(b *pmBlock, qc *block.QuorumCert) (bool, error) {
	var blkID, txsRoot, stateRoot, msgHash meter.Bytes32
	var blk *block.Block
	var blkType uint32
	var err error
	// genesis does not have qc
	if b.Height == 0 && qc.QCHeight == 0 {
		return true, nil
	}

	if b.ProposedBlockInfo == nil {
		// decode block to get qc
		if blk, err = block.BlockDecodeFromBytes(b.ProposedBlock); err != nil {
			fmt.Println("can not decode block", err)
			return false, errors.New("can not decode proposed block")
		}
		blkType = blk.BlockType()
	} else {
		blk = b.ProposedBlockInfo.ProposedBlock
		blkType = uint32(b.ProposedBlockType)
	}

	txsRoot = blk.Header().TxsRoot()
	stateRoot = blk.Header().StateRoot()
	blkID = blk.ID()

	signMsg := p.csReactor.BuildProposalBlockSignMsg(blkType, uint64(b.Height), &blkID, &txsRoot, &stateRoot)
	p.logger.Debug("BlockMatchQC", "signMsg", signMsg)
	msgHash = p.csReactor.csCommon.Hash256Msg([]byte(signMsg))
	//qc at least has 1 vote signature and they are the same, so compare [0] is good enough
	if bytes.Compare(msgHash.Bytes(), meter.Bytes32(qc.VoterMsgHash).Bytes()) == 0 {
		p.logger.Debug("QC matches block", "msgHash", msgHash.String(), "qc voter Msghash", meter.Bytes32(qc.VoterMsgHash).String())
		return true, nil
	} else {
		p.logger.Warn("QC doesn't matches block", "msgHash", msgHash.String(), "qcVoterMsghash", meter.Bytes32(qc.VoterMsgHash).String())
		return false, nil
	}
}
