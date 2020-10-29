// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

func (p *Pacemaker) proposeBlock(parentBlock *block.Block, height, round uint32, qc *pmQuorumCert, timeout bool) (*ProposedBlockInfo, []byte) {

	var proposalKBlock bool = false
	var powResults *powpool.PowResult
	if (height-p.startHeight) >= p.minMBlocks && !timeout {
		proposalKBlock, powResults = powpool.GetGlobPowPoolInst().GetPowDecision()
	}

	var blockBytes []byte
	var blkInfo *ProposedBlockInfo

	// propose appropriate block info
	if proposalKBlock {
		data := &block.KBlockData{uint64(powResults.Nonce), powResults.Raw}
		rewards := powResults.Rewards
		blkInfo = p.csReactor.BuildKBlock(parentBlock, data, rewards)
	} else {
		blkInfo = p.csReactor.BuildMBlock(parentBlock)
		lastKBlockHeight := blkInfo.ProposedBlock.Header().LastKBlockHeight()
		blockNumber := blkInfo.ProposedBlock.Header().Number()
		if round == 0 || blockNumber == lastKBlockHeight+1 {
			// set committee info
			p.packCommitteeInfo(blkInfo.ProposedBlock)
		}
	}
	p.packQuorumCert(blkInfo.ProposedBlock, qc)
	blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)

	return blkInfo, blockBytes
}

func (p *Pacemaker) proposeStopCommitteeBlock(parentBlock *block.Block, height, round uint32, qc *pmQuorumCert) (*ProposedBlockInfo, []byte) {

	var blockBytes []byte
	var blkInfo *ProposedBlockInfo

	blkInfo = p.csReactor.BuildStopCommitteeBlock(parentBlock)
	p.packQuorumCert(blkInfo.ProposedBlock, qc)
	blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)

	return blkInfo, blockBytes
}

func (p *Pacemaker) packCommitteeInfo(blk *block.Block) {
	committeeInfo := []block.CommitteeInfo{}

	// blk.SetBlockEvidence(ev)
	committeeInfo = p.csReactor.MakeBlockCommitteeInfo(p.csReactor.csCommon.GetSystem(), p.csReactor.curActualCommittee)
	// fmt.Println("committee info: ", committeeInfo)
	blk.SetCommitteeInfo(committeeInfo)
	blk.SetCommitteeEpoch(p.csReactor.curEpoch)

	//Fill new info into block, re-calc hash/signature
	// blk.SetEvidenceDataHash(blk.EvidenceDataHash())
}

func (p *Pacemaker) packQuorumCert(blk *block.Block, qc *pmQuorumCert) {
	blk.SetQC(qc.QC)
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
		msg.TimeoutHeight = ti.height
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
	// SkipSignatureCheck flag: skip check match block
	if p.csReactor.config.SkipSignatureCheck {
		return true, nil
	}

	if b.ProposedBlockInfo == nil {
		// decode block to get qc
		if blk, err = block.BlockDecodeFromBytes(b.ProposedBlock); err != nil {
			fmt.Println("can not decode block", err)
			return false, errors.New("can not decode proposed block")
		}
		blkType = blk.Header().BlockType()
	} else {
		blk = b.ProposedBlockInfo.ProposedBlock
		blkType = uint32(b.ProposedBlockType)
	}

	txsRoot = blk.Header().TxsRoot()
	stateRoot = blk.Header().StateRoot()
	blkID = blk.Header().ID()

	signMsg := p.csReactor.BuildProposalBlockSignMsg(blkType, uint64(b.Height), &blkID, &txsRoot, &stateRoot)
	p.logger.Debug("BlockMatchQC", "signMsg", signMsg)
	msgHash = p.csReactor.csCommon.Hash256Msg([]byte(signMsg))
	//qc at least has 1 vote signature and they are the same, so compare [0] is good enough
	if bytes.Compare(msgHash.Bytes(), meter.Bytes32(qc.VoterMsgHash).Bytes()) == 0 {
		p.logger.Debug("QC matches block", "msgHash", msgHash.String(), "qc voter Msghash", meter.Bytes32(qc.VoterMsgHash).String())
		return true, nil
	} else {
		p.logger.Warn("QC doesn't matches block", "msgHash", msgHash.String(), "qc voter Msghash", meter.Bytes32(qc.VoterMsgHash).String())
		return false, nil
	}
}
