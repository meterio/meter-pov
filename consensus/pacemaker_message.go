package consensus

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

func (p *Pacemaker) proposeBlock(parentBlock *block.Block, height, round uint64, qc *pmQuorumCert, allowEmptyBlock bool) (*ProposedBlockInfo, []byte) {
	// XXX: propose an empty block by default. Will add option --consensus.allow_empty_block = false
	// force it to true at this time
	allowEmptyBlock = true

	//check POW pool and TX pool, propose kblock/mblock accordingly
	// The first MBlock must be generated because committee info is in this block
	proposalKBlock := false
	var powResults *powpool.PowResult
	if round >= 5 {
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
		if round == 0 {
			// set committee info
			p.packCommitteeInfo(blkInfo.ProposedBlock)
		}
	}
	p.packQuorumCert(blkInfo.ProposedBlock, qc)
	blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)

	return blkInfo, blockBytes
}

func (p *Pacemaker) proposeStopCommitteeBlock(parentBlock *block.Block, height, round uint64, qc *pmQuorumCert) (*ProposedBlockInfo, []byte) {

	var blockBytes []byte
	var blkInfo *ProposedBlockInfo

	blkInfo = p.csReactor.BuildStopCommitteeBlock(parentBlock)
	p.packQuorumCert(blkInfo.ProposedBlock, qc)
	blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)

	return blkInfo, blockBytes
}

func (p *Pacemaker) packCommitteeInfo(blk *block.Block) error {
	committeeInfo := []block.CommitteeInfo{}
	// only round 0 Mblock contains the following info
	system := p.csReactor.csCommon.system
	blk.SetSystemBytes(system.ToBytes())
	// fmt.Println("system: ", system)

	params := p.csReactor.csCommon.params
	paramsBytes, _ := params.ToBytes()
	blk.SetParamsBytes(paramsBytes)
	// fmt.Println("params: ", params)

	// blk.SetBlockEvidence(ev)
	committeeInfo = p.csReactor.MakeBlockCommitteeInfo(system, p.csReactor.curActualCommittee)
	// fmt.Println("committee info: ", committeeInfo)
	blk.SetCommitteeInfo(committeeInfo)
	blk.SetCommitteeEpoch(p.csReactor.curEpoch)

	//Fill new info into block, re-calc hash/signature
	// blk.SetEvidenceDataHash(blk.EvidenceDataHash())
	return nil
}

func (p *Pacemaker) packQuorumCert(blk *block.Block, qc *pmQuorumCert) error {
	blk.SetQC(qc.QC)
	return nil
}

func (p *Pacemaker) BuildProposalMessage(height, round uint64, bnew *pmBlock, tc *PMTimeoutCert) (*PMProposalMessage, error) {
	blockBytes := bnew.ProposedBlock

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    int64(height),
		Round:     int(round),
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_PROPOSAL_BLOCK,

		// MsgSubType: msgSubType,
		EpochID: p.csReactor.curEpoch,
	}

	parentHeight := uint64(0)
	parentRound := uint64(0)
	if bnew.Parent != nil {
		parentHeight = bnew.Parent.Height
		parentRound = bnew.Parent.Round
	}
	msg := &PMProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		ParentHeight: parentHeight,
		ParentRound:  parentRound,

		ProposerID:        crypto.FromECDSAPub(&p.csReactor.myPubKey),
		CSProposerPubKey:  p.csReactor.csCommon.system.PubKeyToBytes(p.csReactor.csCommon.PubKey),
		KBlockHeight:      int64(p.csReactor.lastKBlockHeight),
		SignOffset:        MSG_SIGN_OFFSET_DEFAULT,
		SignLength:        MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:      len(blockBytes),
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
func (p *Pacemaker) BuildVoteForProposalMessage(proposalMsg *PMProposalMessage) (*PMVoteForProposalMessage, error) {
	proposerPubKey, err := crypto.UnmarshalPubkey(proposalMsg.ProposerID)
	if err != nil {
		p.logger.Error("ummarshal proposer public key of sender failed ")
		return nil, err
	}
	ch := proposalMsg.CSMsgCommonHeader

	offset := proposalMsg.SignOffset
	length := proposalMsg.SignLength
	signMsg := p.csReactor.BuildProposalBlockSignMsg(*proposerPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))

	sign := p.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := p.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    ch.Height,
		Round:     ch.Round,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_VOTE_FOR_PROPOSAL,
	}

	index := p.csReactor.GetCommitteeMemberIndex(p.csReactor.myPubKey)
	msg := &PMVoteForProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		VoterID:           crypto.FromECDSAPub(&p.csReactor.myPubKey),
		CSVoterPubKey:     p.csReactor.csCommon.system.PubKeyToBytes(p.csReactor.csCommon.PubKey),
		VoterSignature:    p.csReactor.csCommon.system.SigToBytes(sign), //TBD
		VoterIndex:        int64(index),
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
func (p *Pacemaker) BuildNewViewMessage(nextHeight, nextRound uint64, qcHigh *pmQuorumCert, reason NewViewReason, ti *PMRoundTimeoutInfo) (*PMNewViewMessage, error) {

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    int64(nextHeight),
		Round:     int(nextRound),
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_VOTE_FOR_PROPOSAL,
	}

	index := p.csReactor.GetCommitteeMemberIndex(p.csReactor.myPubKey)

	signMsg := p.BuildNewViewSignMsg(p.csReactor.myPubKey, reason, nextHeight, nextRound, qcHigh.QC)

	offset := uint32(0)
	length := uint32(len(signMsg))
	sign := p.csReactor.csCommon.SignMessage([]byte(signMsg), offset, length)
	msgHash := p.csReactor.csCommon.Hash256Msg([]byte(signMsg), offset, length)

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
		PeerIndex:         index,
		SignedMessageHash: msgHash,
		PeerSignature:     p.csReactor.csCommon.system.SigToBytes(sign),
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

func (p *Pacemaker) BuildNewViewSignMsg(pubKey ecdsa.PublicKey, reason NewViewReason, height, round uint64, qc *block.QuorumCert) string {
	return fmt.Sprintf("New View Message: Peer:%s Height:%v Round:%v Reason:%v QC:(%d,%d,%v,%v)",
		hex.EncodeToString(crypto.FromECDSAPub(&pubKey)), height, round, reason, qc.QCHeight, qc.QCRound, qc.EpochID, hex.EncodeToString(qc.VoterAggSig))
}

func (p *Pacemaker) BuildQueryProposalMessage(height, round, epochID uint64, retAddr types.NetAddress) (*PMQueryProposalMessage, error) {
	cmnHdr := ConsensusMsgCommonHeader{
		Height:    0,
		Round:     0,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_PACEMAKER_QUERY_PROPOSAL,

		// MsgSubType: msgSubType,
		EpochID: epochID,
	}

	msg := &PMQueryProposalMessage{
		CSMsgCommonHeader: cmnHdr,
		Height:            height,
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
	p.logger.Debug("Built Query Proposal Message", "round", msg.Round, "msg", msg.String(), "timestamp", msg.CSMsgCommonHeader.Timestamp)

	return msg, nil
}
