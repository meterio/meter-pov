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
	// "bytes"
	"fmt"
	"time"

	//"unsafe"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/powpool"

	//"github.com/dfinlab/meter/chain"
	//"github.com/dfinlab/meter/runtime"
	//"github.com/dfinlab/meter/state"
	//"github.com/dfinlab/meter/tx"
	//"github.com/dfinlab/meter/xenv"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

const (

	// FSM of PROPOSER
	COMMITTEE_PROPOSER_INIT       = byte(0x01)
	COMMITTEE_PROPOSER_PROPOSED   = byte(0x02)
	COMMITTEE_PROPOSER_NOTARYSENT = byte(0x03)
	COMMITTEE_PROPOSER_COMMITED   = byte(0x04)

	PROPOSER_THRESHOLD_TIMER_TIMEOUT = 3 * time.Second //wait for reach 2/3 consensus timeout

	PROPOSE_MSG_SUBTYPE_KBLOCK = byte(0x01)
	PROPOSE_MSG_SUBTYPE_MBLOCK = byte(0x02)
)

type ConsensusProposer struct {
	node_id   uint32
	EpochID   uint64 // unique identifier for this consensus session
	state     byte
	csReactor *ConsensusReactor //global reactor info

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
	cp.EpochID = conR.curEpoch

	cp.proposalVoterBitArray = cmn.NewBitArray(conR.committeeSize)
	cp.notaryVoterBitArray = cmn.NewBitArray(conR.committeeSize)

	// form topology,
	/*
		for _, v := range conR.curActualCommittee {
			//skip myself
			if bytes.Equal(crypto.FromECDSAPub(&v.PubKey), crypto.FromECDSAPub(&cp.csReactor.myPubKey)) == true {
				continue
			}
			// initialize PeerConn
			p := newConsensusPeer(v.NetAddr.IP, v.NetAddr.Port)
			cp.csPeers = append(cp.csPeers, p)
		}
	*/

	/*
		myIndex := conR.GetActualCommitteeMemberIndex(&conR.myPubKey)
		l := myIndex + 1
		r := myIndex + MAX_PEERS
		count := len(conR.curActualCommittee)
		max := count + myIndex - 1
		if r > max {
			r = max
		}
		if l <= max {
			i := l
			for i <= r {
				netAddr := conR.curActualCommittee[i%count].NetAddr
				p := newConsensusPeer(netAddr.IP, netAddr.Port)
				cp.csPeers = append(cp.csPeers, p)
				i++
			}
		}
	*/

	return &cp
}

// send consensus message to all connected peers
func (cp *ConsensusProposer) SendMsg(msg *ConsensusMessage) bool {
	return cp.csReactor.SendMsgToPeers(cp.csPeers, msg)
}

// Move to the init State
func (cp *ConsensusProposer) MoveInitState(curState byte, sendNewRoundMsg bool) bool {
	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound
	cp.csReactor.logger.Info("Move to init state of proposer",
		"curHeight", curHeight, "curRound", curRound,
		"curState", curState,
		"actualComitteeSize", len(cp.csReactor.curActualCommittee),
		"comitteeSize", len(cp.csReactor.curCommittee.Validators))

	// clean up all data

	if !sendNewRoundMsg {
		cp.csReactor.logger.Info("current state %v, move to state init", curState)
		cp.state = COMMITTEE_PROPOSER_INIT
		return true
	}

	curActualSize := len(cp.csReactor.curActualCommittee)
	if curActualSize == 0 {
		cp.csReactor.logger.Error("ActualCommittee len is 0")
		return false
	}

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
		CurProposer: crypto.FromECDSAPub(&cp.csReactor.curActualCommittee[curRound%curActualSize].PubKey),
		NewProposer: crypto.FromECDSAPub(&cp.csReactor.curActualCommittee[(curRound+1)%curActualSize].PubKey),
	}

	// sign message
	msgSig, err := cp.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cp.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)

	// state to init & send move to next round
	// fmt.Println("msg: %v", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)

	cp.csReactor.UpdateRound(cp.csReactor.curRound + 1)

	// fmt.Println("current state %v, move to state init", curState)
	cp.state = COMMITTEE_PROPOSER_INIT
	return true
}

// Check Kblock in available in
func (cp *ConsensusProposer) CheckKblockDecision() (bool, *powpool.PowResult) {
	decided, results := powpool.GetGlobPowPoolInst().GetPowDecision()

	return decided, results
}

// Proposer needs to chek POW pool and TX pool, make decision
// to propose the Kblock or mBlock.
func (cp *ConsensusProposer) ProposalBlockMsg(proposalEmptyBlock bool) bool {

	// XXX: propose an empty block by default. Will add option --consensus.create_empty_block = false
	// force it to true at this time
	proposalEmptyBlock = true

	cp.csPeers = make([]*ConsensusPeer, 0)
	peers, _ := cp.csReactor.GetMyPeers()
	for _, p := range peers {
		cp.csPeers = append(cp.csPeers, p)
		// fmt.Println("added peer: ", p.netAddr.IP, ":", p.netAddr.Port)
	}

	//check POW pool and TX pool, propose kblock/mblock/no need to propose.
	// The first MBlock must be generated because committee info is in this block
	proposalKBlock := false
	powResults := &powpool.PowResult{}
	if cp.csReactor.curRound != 0 {
		proposalKBlock, powResults = cp.CheckKblockDecision()
	}

	// XXX: only generate empty Block at initial test
	// check TX pool and POW pool, decide to go which block
	// if there is nothing to do, move to next round.
	if proposalKBlock {
		//kblock, err := cp.buildKBlock(cp.csReactor.kBlockData)
		kblock, err := cp.buildKBlock(&block.KBlockData{uint64(powResults.Nonce), powResults.Raw},
			powResults.Rewards)
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
		EpochID:    cp.EpochID,
	}

	msg := &ProposalBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:       crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		CSProposerPubKey: cp.csReactor.csCommon.system.PubKeyToBytes(cp.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(cp.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(mblock),
		ProposedBlock:    mblock,
	}

	// sign message
	msgSig, err := cp.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cp.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cp.csReactor.logger.Debug("Generated Proposal Block Message for MBlock", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)
	var m ConsensusMessage = msg
	cp.state = COMMITTEE_PROPOSER_PROPOSED
	cp.SendMsg(&m)

	//timeout function
	proposalExpire := func() {
		cp.csReactor.logger.Warn("Reach 2/3 votes of proposal expired", "comitteeSize", cp.csReactor.committeeSize, "voterCount", cp.proposalVoterNum)

		//revert to checkpoint
		best := cp.csReactor.chain.BestBlock()
		state, err := cp.csReactor.stateCreator.NewState(best.Header().StateRoot())
		if err != nil {
			panic(fmt.Sprintf("revert state failed ... %v", err))
		}
		state.RevertTo(cp.curProposedBlockInfo.CheckPoint)

		cp.MoveInitState(cp.state, true)
	}

	cp.proposalThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, func() {
		cp.csReactor.schedulerQueue <- proposalExpire
	})

	return true
}

// Build MBlock for consensus and commit it after consensus
// ConsenusLeader generate the 1st block. With meta data of group
func (cp *ConsensusProposer) buildMBlock(buildEmptyBlock bool) ([]byte, error) {

	blkInfo := cp.csReactor.BuildMBlock(cp.csReactor.chain.BestBlock())
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
		EpochID:    cp.EpochID,
	}

	msg := &ProposalBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:       crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		CSProposerPubKey: cp.csReactor.csCommon.system.PubKeyToBytes(cp.csReactor.csCommon.PubKey),
		KBlockHeight:     int64(cp.csReactor.lastKBlockHeight),
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(kblock),
		ProposedBlock:    kblock,
	}

	// sign message
	msgSig, err := cp.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cp.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cp.csReactor.logger.Debug("Generate Proposal Block Message for KBlock", "height", msg.CSMsgCommonHeader.Height, "timestamp", msg.CSMsgCommonHeader.Timestamp)

	var m ConsensusMessage = msg
	cp.state = COMMITTEE_PROPOSER_PROPOSED
	cp.SendMsg(&m)

	//timeout function
	proposalExpire := func() {
		cp.csReactor.logger.Warn("Reach 2/3 votes of proposal expired", "comitteeSize", cp.csReactor.committeeSize, "voterCount", cp.proposalVoterNum)

		//revert to checkpoint
		best := cp.csReactor.chain.BestBlock()
		state, err := cp.csReactor.stateCreator.NewState(best.Header().StateRoot())
		if err != nil {
			panic(fmt.Sprintf("revert state failed ... %v", err))
		}
		state.RevertTo(cp.curProposedBlockInfo.CheckPoint)

		cp.MoveInitState(cp.state, true)
	}
	cp.proposalThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, func() {
		cp.csReactor.schedulerQueue <- proposalExpire
	})

	return true
}

// ConsenusLeader generate the 1st block. With meta data of group
func (cp *ConsensusProposer) buildKBlock(data *block.KBlockData, rewards []powpool.PowReward) ([]byte, error) {
	blkInfo := cp.csReactor.BuildKBlock(cp.csReactor.chain.BestBlock(), data, rewards)
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
		EpochID:   cp.EpochID,
	}

	msg := &NotaryBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:    crypto.FromECDSAPub(&cp.csReactor.myPubKey),
		CommitteeSize: cp.csReactor.committeeSize,

		SignOffset:        MSG_SIGN_OFFSET_DEFAULT,
		SignLength:        MSG_SIGN_LENGTH_DEFAULT,
		VoterBitArray:     *cp.proposalVoterBitArray,
		VoterAggSignature: cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig),
	}

	// sign message
	msgSig, err := cp.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cp.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
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
		cp.csReactor.logger.Warn("state machine incorrect", "expected", "PROPOSED", "actual", cp.state)
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

	if cp.csReactor.ValidateCMheaderSig(&ch, vote4ProposalMsg.SigningHash().Bytes()) == false {
		cp.csReactor.logger.Error("Signature validate failed")
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
		if cp.csReactor.config.SkipSignatureCheck == true {
			cp.csReactor.logger.Error("but SkipSignatureCheck is true, continue ...")
		} else {
			return false
		}
	}

	// 2. add src to bitArray.
	cp.proposalVoterNum++
	cp.proposalVoterBitArray.SetIndex(index, true)

	cp.proposalVoterSig = append(cp.proposalVoterSig, sig)
	cp.proposalVoterPubKey = append(cp.proposalVoterPubKey, pubKey)
	cp.proposalVoterMsgHash = append(cp.proposalVoterMsgHash, msgHash)

	// 3. if the totoal vote > 2/3, move to NotarySent state
	if MajorityTwoThird(cp.proposalVoterNum, cp.csReactor.committeeSize) &&
		cp.state == COMMITTEE_PROPOSER_PROPOSED {

		cp.proposalVoterAggSig = cp.csReactor.csCommon.AggregateSign(cp.proposalVoterSig)

		cp.proposalThresholdTimer.Stop()

		//send out notary
		cp.GenerateNotaryBlockMsg()
		cp.state = COMMITTEE_PROPOSER_NOTARYSENT

		//timeout function
		notaryBlockExpire := func() {
			cp.csReactor.logger.Warn("reach 2/3 vote of notaryBlock expired ...", "comitteeSize", cp.csReactor.committeeSize, "receivedVotesOfNotary", cp.notaryVoterNum)

			//revert to checkpoint
			best := cp.csReactor.chain.BestBlock()
			state, err := cp.csReactor.stateCreator.NewState(best.Header().StateRoot())
			if err != nil {
				panic(fmt.Sprintf("revert the state faild ... %v", err))
			}
			state.RevertTo(cp.curProposedBlockInfo.CheckPoint)

			cp.MoveInitState(cp.state, true)
		}

		cp.notaryThresholdTimer = time.AfterFunc(PROPOSER_THRESHOLD_TIMER_TIMEOUT, func() {
			cp.csReactor.schedulerQueue <- notaryBlockExpire
		})
	}
	return true
}

func (cp *ConsensusProposer) ProcessVoteForNotary(vote4NotaryMsg *VoteForNotaryMessage, src *ConsensusPeer) bool {
	//logger := cp.csReactor.Logger

	// only process Vote Notary in state NotarySent
	if cp.state != COMMITTEE_PROPOSER_NOTARYSENT {
		cp.csReactor.logger.Warn("state machine incorrect", "expected", "NOTARYSENT", "actual", cp.state)
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

	if cp.csReactor.ValidateCMheaderSig(&ch, vote4NotaryMsg.SigningHash().Bytes()) == false {
		cp.csReactor.logger.Error("Signature validate failed")
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
		if cp.csReactor.config.SkipSignatureCheck == true {
			cp.csReactor.logger.Error("but SkipSignatureCheck is true, continue ...")
		} else {
			return false
		}
	}

	// 2. add src to bitArray.
	cp.notaryVoterNum++
	cp.notaryVoterBitArray.SetIndex(index, true)

	cp.notaryVoterSig = append(cp.notaryVoterSig, sig)
	cp.notaryVoterPubKey = append(cp.notaryVoterPubKey, pubKey)
	cp.notaryVoterMsgHash = append(cp.notaryVoterMsgHash, msgHash)

	// XXX:  Yang: Hack here +2 to pass 2/3
	// 3. if the totoal vote > 2/3, move to Commit state

	if MajorityTwoThird(cp.notaryVoterNum, cp.csReactor.committeeSize) &&
		cp.state == COMMITTEE_PROPOSER_NOTARYSENT {

		//save all group info as meta data
		cp.state = COMMITTEE_PROPOSER_COMMITED
		cp.notaryThresholdTimer.Stop()

		// Now commit this block
		//logger.Info("")
		cp.csReactor.logger.Info(`
===========================================================
Block proposal is approved, commit now ...
Move to next height
===========================================================`,
			"height", cp.csReactor.curHeight, "round", cp.csReactor.curRound)
		//logger.Info("")

		// Aggregate signatures
		cp.notaryVoterAggSig = cp.csReactor.csCommon.AggregateSign(cp.notaryVoterSig)
		cp.proposalVoterAggSig = cp.csReactor.csCommon.AggregateSign(cp.proposalVoterSig)

		/******
		fmt.Println("VoterMsgHash", cp.proposalVoterMsgHash)
		for _, p := range cp.proposalVoterPubKey {
			fmt.Println("pubkey:::", cp.csReactor.csCommon.system.PubKeyToBytes(p))
		}
		fmt.Println("aggrsig", cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig), "system", cp.csReactor.csCommon.system.ToBytes())
		******/

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

			//revert to checkpoint
			best := cp.csReactor.chain.BestBlock()
			state, err := cp.csReactor.stateCreator.NewState(best.Header().StateRoot())
			if err != nil {
				panic(fmt.Sprintf("revert the state faild ... %v", err))
			}
			state.RevertTo(cp.curProposedBlockInfo.CheckPoint)

			// wait 5 send and move to next round
			time.AfterFunc(WHOLE_NETWORK_BLOCK_SYNC_TIME, func() {
				cp.csReactor.schedulerQueue <- func() {
					cp.MoveInitState(cp.state, true)
				}
			})

			return true
		}

		// blcok is successfully commited
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

		} else {
			// mblock, wait for 5 s and sends move to next round msg
			time.AfterFunc(WHOLE_NETWORK_BLOCK_SYNC_TIME, func() {
				cp.csReactor.schedulerQueue <- func() {
					cp.MoveInitState(cp.state, true)
				}
			})
			return true
		}
	} else {
		// received VoteForNotary but 2/3 is not reached, keep waiting.
		return true
	}

	// cp.csReactor.UpdateRound(cp.csReactor.curRound + 1)
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
