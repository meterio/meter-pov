package consensus

import (
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/powpool"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

func (p *Pacemaker) BuildProposalMessage(allowEmptyBlock bool) (*PMProposalMessage, error) {
	// XXX: propose an empty block by default. Will add option --consensus.allow_empty_block = false
	// force it to true at this time
	allowEmptyBlock = true

	//check POW pool and TX pool, propose kblock/mblock/no need to propose.
	// The first MBlock must be generated because committee info is in this block
	proposalKBlock := false
	var powResults *powpool.PowResult
	if p.csReactor.curRound != 0 {
		proposalKBlock, powResults = powpool.GetGlobPowPoolInst().GetPowDecision()
	}

	// generate appropiate block message
	if proposalKBlock {
		return p.buildKBlockMessage(&block.KBlockData{uint64(powResults.Nonce), powResults.Raw},
			powResults.Rewards, allowEmptyBlock)
	} else {
		return p.buildMBlockMessage(allowEmptyBlock)
	}
}

// buildMBlockMsg will build pacemaker proposal message
func (p *Pacemaker) buildMBlockMessage(buildEmptyBlock bool) (*PMProposalMessage, error) {
	//TODO: use this
	buildEmptyBlock = true
	blkInfo := p.csReactor.BuildMBlock()
	mblockBytes := block.BlockEncodeBytes(blkInfo.ProposedBlock)

	//TODO: save to local
	/*
		cp.curProposedBlockInfo = *blkInfo
		cp.curProposedBlock = blkBytes
		cp.curProposedBlockType = PROPOSE_MSG_SUBTYPE_MBLOCK
	*/

	curHeight := p.csReactor.curHeight
	curRound := p.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: PROPOSE_MSG_SUBTYPE_MBLOCK,
		// TODO: set epochID EpochID:    p.EpochID,
	}

	msg := &PMProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:       crypto.FromECDSAPub(&p.csReactor.myPubKey),
		CSProposerPubKey: p.csReactor.csCommon.system.PubKeyToBytes(p.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(p.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(mblockBytes),
		ProposedBlock:    mblockBytes,
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Generated Proposal Block Message for MBlock", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)
	return msg, nil
}

// Propose the Kblock
func (p *Pacemaker) buildKBlockMessage(data *block.KBlockData, rewards []powpool.PowReward, buildEmptyBlock bool) (*PMProposalMessage, error) {
	// TODO: use this
	buildEmptyBlock = true
	blkInfo := p.csReactor.BuildKBlock(data, rewards)
	kblockBytes := block.BlockEncodeBytes(blkInfo.ProposedBlock)

	//TODO: save to local
	/*
		cp.curProposedBlockInfo = *blkInfo
		cp.curProposedBlock = blkBytes
		cp.curProposedBlockType = PROPOSE_MSG_SUBTYPE_KBLOCK
	*/
	curHeight := p.csReactor.curHeight
	curRound := p.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: PROPOSE_MSG_SUBTYPE_KBLOCK,
		// TODO: set epochID EpochID:    p.EpochID,
	}

	msg := &PMProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:       crypto.FromECDSAPub(&p.csReactor.myPubKey),
		CSProposerPubKey: p.csReactor.csCommon.system.PubKeyToBytes(p.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(p.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(kblockBytes),
		ProposedBlock:    kblockBytes,
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Generate Proposal Block Message for KBlock", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)

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

	curHeight := p.csReactor.curHeight
	curRound := p.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
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
	p.logger.Debug("Generate Voter For Proposal Message", "msg", msg.String())
	return msg, nil
}

// BuildVoteForProposalMsg build VFP message for proposal
func (p *Pacemaker) BuildNewViewMessage(qcHigh *block.QuorumCert) (*PMNewViewMessage, error) {

	curHeight := p.csReactor.curHeight
	curRound := p.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_VOTE_FOR_PROPOSAL,
	}

	msg := &PMNewViewMessage{
		CSMsgCommonHeader: cmnHdr,

		QCHeight: qcHigh.QCHeight,
		QCRound:  qcHigh.QCRound,
		QCHigh:   qcHigh.ToBytes(),
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Generate Voter For Proposal Message", "msg", msg.String())
	return msg, nil
}
