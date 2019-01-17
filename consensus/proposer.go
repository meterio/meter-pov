/*****
 Proposer Functionalities:
    1) add peers
    2) generates the mblock proposal
    3) receive vote
    4) notary the proposal, then commit the block.
***/

package consensus

import (
	//    "errors"
	"bytes"
	"fmt"
	"time"

	//"unsafe"
	"github.com/vechain/thor/block"
	//"github.com/vechain/thor/chain"
	//"github.com/vechain/thor/runtime"
	//"github.com/vechain/thor/state"
	//"github.com/vechain/thor/tx"
	//"github.com/vechain/thor/xenv"

	crypto "github.com/ethereum/go-ethereum/crypto"
	bls "github.com/vechain/thor/crypto/multi_sig"
	cmn "github.com/vechain/thor/libs/common"
)

const (
	// FSM of PROPOSER
	COMMITTEE_PROPOSER_INIT       = byte(0x01)
	COMMITTEE_PROPOSER_PROPOSED   = byte(0x02)
	COMMITTEE_PROPOSER_NOTARYSENT = byte(0x03)
	COMMITTEE_PROPOSER_COMMITED   = byte(0x04)

	PROPOSER_THRESHOLD_TIMER_TIMEOUT = 1 * time.Second //wait for reach 2/3 consensus timeout

	PROPOSE_MSG_SUBTYPE_KBLOCK = byte(0x01)
	PROPOSE_MSG_SUBTYPE_MBLOCK = byte(0x02)
)

type ConsensusProposer struct {
	node_id     uint32
	CommitteeID uint32 // unique identifier for this consensus session
	state       byte
	csReactor   *ConsensusReactor //global reactor info

	// local copy of proposed block
	curProposedBlockInfo ProposedBlockInfo //data structure
	curProposedBlock     []byte            // byte slice block
	curProposedBlockType byte

	//signature data , slice signature and public key must be match
	proposalVoterBitArray *cmn.BitArray
	proposalVoterSig      []bls.Signature
	proposalVoterPubKey   []bls.PublicKey
	proposalVoterMsgHash  [][32]byte
	proposalVoterAggSig   bls.Signature
	proposalVoterNum      int
	//
	notaryVoterBitArray *cmn.BitArray
	notaryVoterSig      []bls.Signature
	notaryVoterPubKey   []bls.PublicKey
	notaryVoterMsgHash  [][32]byte
	notaryVoterAggSig   bls.Signature
	notaryVoterNum      int

	proposalThresholdTimer *time.Timer      // 2/3 voting timer
	notaryThresholdTimer   *time.Timer      // notary 2/3 vote timer
	csPeers                []*ConsensusPeer // consensus message peers
}

//New CommitteeLeader
func NewCommitteeProposer(conR *ConsensusReactor) *ConsensusProposer {
	var cp ConsensusProposer

	// initialize the ConsenusLeader
	//cl.consensus_id = conR.consensus_id
	cp.state = COMMITTEE_PROPOSER_INIT
	cp.csReactor = conR

	cp.proposalVoterBitArray = cmn.NewBitArray(conR.committeeSize)
	cp.notaryVoterBitArray = cmn.NewBitArray(conR.committeeSize)

	// form topology,
	for _, v := range conR.curActualCommittee {
		//skip myself
		if bytes.Equal(crypto.FromECDSAPub(&v.PubKey), crypto.FromECDSAPub(&cp.csReactor.myPubKey)) == true {
			continue
		}
		// initialize PeerConn
		p := newConsensusPeer(v.NetAddr.IP, v.NetAddr.Port)
		cp.csPeers = append(cp.csPeers, p)
	}

	return &cp
}

// send consensus message to all connected peers
func (cp *ConsensusProposer) SendMsg(msg *ConsensusMessage) bool {

	if len(cp.csPeers) == 0 {
		cp.csReactor.sendConsensusMsg(msg, nil)
		return true
	}

	for _, p := range cp.csPeers {
		//p.sendConsensusMsg(msg)
		cp.csReactor.sendConsensusMsg(msg, p)
	}

	return true
}

// Move to the init State
func (cp *ConsensusProposer) MoveInitState(curState byte, sendNewRoundMsg bool) bool {
	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound
	cp.csReactor.logger.Info("Move to init state of proposer",
		"curHeight", curHeight, "curRound", curRound,
		"curState", curState,
		"comitteeSize", len(cp.csReactor.curActualCommittee),
		"comitteeSize", len(cp.csReactor.curCommittee.Validators))

	// clean up all data

	if !sendNewRoundMsg {
		cp.csReactor.logger.Info("current state %v, move to state init", curState)
		cp.state = COMMITTEE_PROPOSER_INIT
		return true
	}

	/*********
	if len(cp.csReactor.curActualCommittee) == 0 {
		cp.csReactor.logger.Error("ActualCommittee len is 0")
		return false
	}
	***********/
	curActualSize := len(cp.csReactor.curCommittee.Validators) //Hack here, should use curActualCommittee
	msg := &MoveNewRoundMessage{
		CSMsgCommonHeader: ConsensusMsgCommonHeader{
			Height:    curHeight,
			Round:     curRound,
			Sender:    crypto.FromECDSAPub(&cp.csReactor.myPubKey),
			Timestamp: time.Now(),
			MsgType:   CONSENSUS_MSG_MOVE_NEW_ROUND,
		},

		Height:   curHeight,
		CurRound: curRound,
		NewRound: curRound + 1,
		//CurProposer: cp.csReactor.curActualCommittee[curRound].PubKey,
		//NewProposer: cp.csReactor.curActualCommittee[curRound+1].PubKey,
		CurProposer: crypto.FromECDSAPub(&cp.csReactor.curCommittee.Validators[curRound%curActualSize].PubKey),
		NewProposer: crypto.FromECDSAPub(&cp.csReactor.curCommittee.Validators[(curRound+1)%curActualSize].PubKey),
	}

	// state to init & send move to next round
	// fmt.Println("msg: %v", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	// fmt.Println("current state %v, move to state init", curState)
	cp.state = COMMITTEE_PROPOSER_INIT
	return true
}

// Check Kblock in available in
func (cp *ConsensusProposer) CheckKblock() bool {
	return cp.csReactor.kBlockData != nil
}

// Proposer needs to chek POW pool and TX pool, make decision
// to propose the Kblock or mBlock.
func (cp *ConsensusProposer) ProposalBlockMsg(proposalEmptyBlock bool) bool {
	// XXX: propose an empty block by default. Will add option --consensus.create_empty_block = false
	// force it to true at this time
	proposalEmptyBlock = true

	//check POW pool and TX pool, propose kblock/mblock/no need to propose.
	proposalKBlock := cp.CheckKblock()

	// XXX: only generate empty Block at initial test
	// check TX pool and POW pool, decide to go which block
	// if there is nothing to do, move to next round.
	if proposalKBlock {
		kblock, err := cp.buildKBlock(cp.csReactor.kBlockData)
		if err != nil {
			cp.csReactor.logger.Error("build Kblock failed ...")
			return false
		}
		cp.csReactor.kBlockData = nil
		cp.GenerateKBlockMsg(kblock)

	} else {

		mblock, err := cp.buildMBlock(proposalEmptyBlock)
		if err != nil {
			cp.csReactor.logger.Error("build Mblock failed ...")
			return false
		}

		cp.GenerateMBlockMsg(mblock)
	}

	return true
}

//XXX: ConsensusLeader always propose the 1st block. With meta data of group
func (cp *ConsensusProposer) GenerateMBlockMsg(mblock []byte) bool {

	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: PROPOSE_MSG_SUBTYPE_MBLOCK,
	}

	msg := &ProposalBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		CommitteeID:      cp.CommitteeID,
		ProposerID:       crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		CSProposerPubKey: cp.csReactor.csCommon.system.PubKeyToBytes(cp.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(cp.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(mblock),
		ProposedBlock:    mblock,
	}

	cp.csReactor.logger.Debug("Generated Proposal Block Message for MBlock", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	cp.state = COMMITTEE_PROPOSER_PROPOSED

	//timeout function
	proposalExpire := func() {
		cp.csReactor.logger.Warn("Reach 2/3 votes of proposal expired", "comitteeSize", cp.csReactor.committeeSize, "voterCount", cp.proposalVoterNum)
		cp.MoveInitState(cp.state, true)
	}
	cp.proposalThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, proposalExpire)

	return true
}

// Build MBlock for consensus and commit it after consensus
// ConsenusLeader generate the 1st block. With meta data of group
func (cp *ConsensusProposer) buildMBlock(buildEmptyBlock bool) ([]byte, error) {

	blkInfo := cp.csReactor.BuildMBlock()
	blkBytes := block.BlockEncodeBytes(blkInfo.ProposedBlock)

	//save to local
	cp.curProposedBlockInfo = *blkInfo
	cp.curProposedBlock = blkBytes
	cp.curProposedBlockType = PROPOSE_MSG_SUBTYPE_MBLOCK

	return blkBytes, nil
}

// Propose the Kblock
func (cp *ConsensusProposer) GenerateKBlockMsg(kblock []byte) bool {

	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: PROPOSE_MSG_SUBTYPE_KBLOCK,
	}

	msg := &ProposalBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		CommitteeID:      cp.CommitteeID,
		ProposerID:       crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		CSProposerPubKey: cp.csReactor.csCommon.system.PubKeyToBytes(cp.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(cp.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(kblock),
		ProposedBlock:    kblock,
	}

	cp.csReactor.logger.Debug("Generate Proposal Block Message for KBlock", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)

	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	cp.state = COMMITTEE_PROPOSER_PROPOSED

	//timeout function
	proposalExpire := func() {
		cp.csReactor.logger.Warn("Reach 2/3 votes of proposal expired", "comitteeSize", cp.csReactor.committeeSize, "voterCount", cp.proposalVoterNum)
		cp.MoveInitState(cp.state, true)
	}
	cp.proposalThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, proposalExpire)

	return true
}

// ConsenusLeader generate the 1st block. With meta data of group
func (cp *ConsensusProposer) buildKBlock(data *block.KBlockData) ([]byte, error) {
	blkInfo := cp.csReactor.BuildKBlock(data)
	blkBytes := block.BlockEncodeBytes(blkInfo.ProposedBlock)

	//save to local
	cp.curProposedBlockInfo = *blkInfo
	cp.curProposedBlock = blkBytes
	cp.curProposedBlockType = PROPOSE_MSG_SUBTYPE_KBLOCK

	return blkBytes, nil
}

// After blockproposal vote > 2/3, proposer generates NotaryBlock
func (cp *ConsensusProposer) GenerateNotaryBlockMsg() bool {

	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_NOTARY_BLOCK,
	}

	msg := &NotaryBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:    crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		CommitteeID:   cp.CommitteeID,
		CommitteeSize: cp.csReactor.committeeSize,

		SignOffset:        MSG_SIGN_OFFSET_DEFAULT,
		SignLength:        MSG_SIGN_LENGTH_DEFAULT,
		VoterBitArray:     *cp.proposalVoterBitArray,
		VoterAggSignature: cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig),
	}

	cp.csReactor.logger.Debug("Generate Notary Block Message", "msg", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	cp.state = COMMITTEE_PROPOSER_NOTARYSENT

	return true
}

//
func (cp *ConsensusProposer) ProcessVoteForProposal(vote4ProposalMsg *VoteForProposalMessage, src *ConsensusPeer) bool {
	// only process Vote in state proposed
	if cp.state < COMMITTEE_PROPOSER_PROPOSED {
		cp.csReactor.logger.Error("state machine incorrect", "expected", "PROPOSED", "actual", cp.state)
		return false
	}

	// valid the common header first
	/****
	vote4ProposalMsg, ok := interface{}(vote).(VoteForProposalMessage)
	if ok != false {
		fmt.Println("Message type is not VoteForProposalMessage")
		return false
	}
	****/
	ch := vote4ProposalMsg.CSMsgCommonHeader
	if !cp.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_VOTE_FOR_PROPOSAL {
		cp.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_VOTE_FOR_PROPOSAL")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	senderPubKey, err := crypto.UnmarshalPubkey(ch.Sender)
	if err != nil {
		fmt.Println("ummarshal public key of sender failed ")
		return false
	}
	index := cp.csReactor.GetCommitteeMemberIndex(*senderPubKey)
	if index != vote4ProposalMsg.VoterIndex {
		cp.csReactor.logger.Error("Voter index mismatch", "expected", index, "actual", vote4ProposalMsg.VoterIndex)
		return false
	}

	//so far so good
	// 1. validate voter signature
	myPubKey := cp.csReactor.myPubKey
	signMsg := cp.csReactor.BuildProposalBlockSignMsg(myPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))
	cp.csReactor.logger.Debug("Sign message", "msg", signMsg)

	// validate the message hash
	msgHash := cp.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(MSG_SIGN_OFFSET_DEFAULT), uint32(MSG_SIGN_LENGTH_DEFAULT))
	if msgHash != vote4ProposalMsg.SignedMessageHash {
		cp.csReactor.logger.Error("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := cp.csReactor.csCommon.system.SigFromBytes(vote4ProposalMsg.VoterSignature)
	if err != nil {
		cp.csReactor.logger.Error("get signature failed ...")
		return false
	}

	pubKey, err := cp.csReactor.csCommon.system.PubKeyFromBytes(vote4ProposalMsg.CSVoterPubKey)
	if err != nil {
		cp.csReactor.logger.Error("get PubKey failed ...")
		return false
	}

	valid := bls.Verify(sig, msgHash, pubKey)
	if valid == false {
		cp.csReactor.logger.Error("validate voter signature failed")
		return false
	}

	// 2. add src to bitArray.
	cp.proposalVoterNum++
	cp.proposalVoterBitArray.SetIndex(index, true)

	cp.proposalVoterSig = append(cp.proposalVoterSig, sig)
	cp.proposalVoterPubKey = append(cp.proposalVoterPubKey, pubKey)
	cp.proposalVoterMsgHash = append(cp.proposalVoterMsgHash, msgHash)

	// XXX::: Yang: Hack here +2 to pass 2/3
	// 3. if the totoal vote > 2/3, move to NotarySent state
	//if cp.proposalVoterNum >= cp.csReactor.committeeSize*2/3 &&
	if (cp.proposalVoterNum+1) >= cp.csReactor.committeeSize*2/3 &&
		cp.state == COMMITTEE_PROPOSER_PROPOSED {

		// Now it is OK to aggregate signatures
		cp.proposalVoterAggSig = cp.csReactor.csCommon.AggregateSign(cp.proposalVoterSig)

		fmt.Println("VoterMsgHash", cp.proposalVoterMsgHash)
		for _, p := range cp.proposalVoterPubKey {
			fmt.Println("pubkey:::", cp.csReactor.csCommon.system.PubKeyToBytes(p))
		}
		fmt.Println("aggrsig", cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig), "system", cp.csReactor.csCommon.system.ToBytes())

		//send out notary
		cp.GenerateNotaryBlockMsg()
		cp.state = COMMITTEE_PROPOSER_NOTARYSENT
		cp.proposalThresholdTimer.Stop()

		//timeout function
		notaryBlockExpire := func() {
			cp.csReactor.logger.Warn("reach 2/3 vote of notaryBlock expired ...", "comitteeSize", cp.csReactor.committeeSize, "receivedVotesOfNotary", cp.notaryVoterNum)
			cp.MoveInitState(cp.state, true)
		}

		cp.notaryThresholdTimer = time.AfterFunc(PROPOSER_THRESHOLD_TIMER_TIMEOUT, notaryBlockExpire)
	}
	return true
}

func (cp *ConsensusProposer) ProcessVoteForNotary(vote4NotaryMsg *VoteForNotaryMessage, src *ConsensusPeer) bool {
	//logger := cp.csReactor.Logger

	// only process Vote Notary in state NotarySent
	if cp.state != COMMITTEE_PROPOSER_NOTARYSENT {
		cp.csReactor.logger.Error("state machine incorrect", "expected", "NOTARYSENT", "actual", cp.state)
		return false
	}

	// valid the common header first
	/*****
	vote4NotaryMsg, ok := interface{}(vote).(VoteForNotaryMessage)
	if ok != false {
		fmt.Println("Message type is not VoteForNotaryMessage")
		return false
	}
	******/
	ch := vote4NotaryMsg.CSMsgCommonHeader
	if !cp.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_VOTE_FOR_NOTARY {
		cp.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_VOTE_FOR_NOTARY")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	senderPubKey, err := crypto.UnmarshalPubkey(ch.Sender)
	if err != nil {
		cp.csReactor.logger.Error("ummarshal public key of sender failed ")
		return false
	}
	index := cp.csReactor.GetCommitteeMemberIndex(*senderPubKey)
	if index != vote4NotaryMsg.VoterIndex {
		cp.csReactor.logger.Error("Voter index mismatch %d vs %d", index, vote4NotaryMsg.VoterIndex)
		return false
	}

	//so far so good
	// 1. validate voter signature
	myPubKey := cp.csReactor.myPubKey
	signMsg := cp.csReactor.BuildNotaryBlockSignMsg(myPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))
	cp.csReactor.logger.Debug("Sign message", "msg", signMsg)

	// validate the message hash
	msgHash := cp.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(MSG_SIGN_OFFSET_DEFAULT), uint32(MSG_SIGN_LENGTH_DEFAULT))
	if msgHash != vote4NotaryMsg.SignedMessageHash {
		cp.csReactor.logger.Error("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := cp.csReactor.csCommon.system.SigFromBytes(vote4NotaryMsg.VoterSignature)
	if err != nil {
		cp.csReactor.logger.Error("get signature failed ...")
		return false
	}

	pubKey, err := cp.csReactor.csCommon.system.PubKeyFromBytes(vote4NotaryMsg.CSVoterPubKey)
	if err != nil {
		cp.csReactor.logger.Error("get PubKey failed ...")
		return false
	}

	valid := bls.Verify(sig, msgHash, pubKey)
	if valid == false {
		cp.csReactor.logger.Error("validate voter signature failed")
		return false
	}

	// 2. add src to bitArray.
	cp.notaryVoterNum++
	cp.notaryVoterBitArray.SetIndex(index, true)

	cp.notaryVoterSig = append(cp.notaryVoterSig, sig)
	cp.notaryVoterPubKey = append(cp.notaryVoterPubKey, pubKey)
	cp.notaryVoterMsgHash = append(cp.notaryVoterMsgHash, msgHash)

	// XXX:  Yang: Hack here +2 to pass 2/3
	// 3. if the totoal vote > 2/3, move to Commit state
	//if cp.notaryVoterNum >= cp.csReactor.committeeSize*2/3 &&
	if (cp.notaryVoterNum+1) >= cp.csReactor.committeeSize*2/3 &&
		cp.state == COMMITTEE_PROPOSER_NOTARYSENT {

		//save all group info as meta data
		cp.state = COMMITTEE_PROPOSER_COMMITED
		cp.notaryThresholdTimer.Stop()

		//aggregate signature
		// Aggregate signature here
		cp.notaryVoterAggSig = cp.csReactor.csCommon.AggregateSign(cp.notaryVoterSig)

		// Now commit this block
		//logger.Info("")
		cp.csReactor.logger.Info(`
===========================================================
Block proposal is approved, commit now ...
Move to next height
===========================================================`,
			"height", cp.csReactor.curHeight, "round", cp.csReactor.curRound)
		//logger.Info("")

		// only the block body are filled. Now fill the Evidence / committeeInfo/ Kblock Data if needed
		votingSig := cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig)
		notarizeSig := cp.csReactor.csCommon.system.SigToBytes(cp.notaryVoterAggSig)
		evidence := block.NewEvidence(votingSig, cp.proposalVoterMsgHash, *cp.proposalVoterBitArray,
			notarizeSig, cp.notaryVoterMsgHash, *cp.notaryVoterBitArray)

		blk := cp.curProposedBlockInfo.ProposedBlock
		if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_KBLOCK {
			// XXX fill KBlockData later
			cp.csReactor.finalizeKBlock(blk, evidence)
		} else if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_MBLOCK {
			cp.csReactor.finalizeMBlock(blk, evidence)
		}

		// commit the approved block
		if cp.csReactor.finalizeCommitBlock(&cp.curProposedBlockInfo) == false {
			cp.csReactor.logger.Error("Commit block failed ...")
			goto INIT_STATE
		}

		// blcok is commited
		if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_KBLOCK {

			cp.MoveInitState(cp.state, false)

			// update the lastKBlockHeight since the kblock is handled
			//blk := cp.curProposedBlockInfo.ProposedBlock
			cp.csReactor.UpdateLastKBlockHeight(blk.Header().Number())

			kBlockData, err := blk.GetKBlockData()
			if err != nil {
				panic("can't get KBlockData")
			}
			nonce := kBlockData.Nonce
			height := blk.Header().Number()

			//exit committee first
			cp.csReactor.exitCurCommittee()
			cp.csReactor.ConsensusHandleReceivedNonce(int64(height), nonce, false)
			return true
		}
	}

INIT_STATE:
	//Finally, go to init
	time.Sleep(5 * time.Second)
	if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_KBLOCK {
		cp.MoveInitState(cp.state, false)
	} else if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_MBLOCK {
		cp.MoveInitState(cp.state, true)
	}

	cp.csReactor.UpdateRound(cp.csReactor.curRound + 1)
	return true
}

func (cp *ConsensusProposer) checkHeightAndRound(ch ConsensusMsgCommonHeader) bool {
	if ch.Height != cp.csReactor.curHeight {
		cp.csReactor.logger.Error("Height mismatch!", "curHeight", cp.csReactor.curHeight, "incomingHeight", ch.Height)
		return false
	}

	if ch.Round != cp.csReactor.curRound {
		cp.csReactor.logger.Error("Round mismatch!", "curRound", cp.csReactor.curRound, "incomingRound", ch.Round)
		return false
	}
	return true
}
