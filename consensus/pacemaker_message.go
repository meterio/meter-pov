package consensus

import (
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/powpool"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

func (p *Pacemaker) proposeBlock(height, round uint64, allowEmptyBlock bool) (*ProposedBlockInfo, []byte) {
	//FIXME: use height and round information during pospose

	// XXX: propose an empty block by default. Will add option --consensus.allow_empty_block = false
	// force it to true at this time
	allowEmptyBlock = true

	//check POW pool and TX pool, propose kblock/mblock accordingly
	// The first MBlock must be generated because committee info is in this block
	proposalKBlock := false
	var powResults *powpool.PowResult
	if p.csReactor.curRound != 0 {
		proposalKBlock, powResults = powpool.GetGlobPowPoolInst().GetPowDecision()
	}

	var blockBytes []byte
	var blkInfo *ProposedBlockInfo

	// propose appropriate block info
	if proposalKBlock {
		data := &block.KBlockData{uint64(powResults.Nonce), powResults.Raw}
		rewards := powResults.Rewards
		blkInfo = p.csReactor.BuildKBlock(data, rewards)
		blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)
	} else {
		blkInfo = p.csReactor.BuildMBlock()
		blockBytes = block.BlockEncodeBytes(blkInfo.ProposedBlock)
	}
	return blkInfo, blockBytes
}

func (p *Pacemaker) BuildProposalMessage(height, round uint64, bnew *pmBlock) (*PMProposalMessage, error) {
	var msgSubType byte
	info := bnew.ProposedBlockInfo
	blockBytes := bnew.ProposedBlock

	if info.BlockType == KBlockType {
		msgSubType = PROPOSE_MSG_SUBTYPE_KBLOCK
	} else {
		msgSubType = PROPOSE_MSG_SUBTYPE_MBLOCK
	}

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     int64(height),
		Round:      int(round),
		Sender:     crypto.FromECDSAPub(&p.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: msgSubType,
		// TODO: set epochID EpochID:    p.EpochID,
	}

	parentHeight := uint64(0)
	parentRound := uint64(0)
	if bnew.Parent != nil {
		parentHeight = bnew.Parent.Height
		parentRound = bnew.Parent.Round
	}
	msg := &PMProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		ParentHeight:     parentHeight,
		ParentRound:      parentRound,
		ProposerID:       crypto.FromECDSAPub(&p.csReactor.myPubKey),
		CSProposerPubKey: p.csReactor.csCommon.system.PubKeyToBytes(p.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(p.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(blockBytes),
		ProposedBlock:    blockBytes,
	}

	// sign message
	msgSig, err := p.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		p.logger.Error("Sign message failed", "error", err)
		return nil, err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	p.logger.Debug("Built Proposal Message", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)

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
func (p *Pacemaker) BuildNewViewMessage(qcHigh *QuorumCert) (*PMNewViewMessage, error) {

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
		QCHigh:   p.EncodeQCToBytes(qcHigh),
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
