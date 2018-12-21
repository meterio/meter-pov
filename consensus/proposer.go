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
	"fmt"
	"time"

	//"unsafe"
	"github.com/vechain/thor/block"
	//"github.com/vechain/thor/chain"
	//"github.com/vechain/thor/runtime"
	//"github.com/vechain/thor/state"
	//"github.com/vechain/thor/tx"
	//"github.com/vechain/thor/xenv"

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
	curProposedBlock     []byte
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
		if v.PubKey == cp.csReactor.myPubKey {
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
		if cp.csReactor.sendConsensusMsg(msg, p) {
			fmt.Println("send consnmessage to %v succesfully", p)
		} else {
			fmt.Println("send consnmessage to %v failed", p)
		}
	}

	return true
}

// Move to the init State
func (cp *ConsensusProposer) MoveInitState(curState byte) bool {
	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound
	fmt.Println("curHeight", curHeight, "curRound", curRound)
	fmt.Println("ActualCommittee size", len(cp.csReactor.curActualCommittee))
	fmt.Println("committee size", len(cp.csReactor.curCommittee.Validators))
	/*********
	if len(cp.csReactor.curActualCommittee) == 0 {
		fmt.Println("ActualCommittee len is 0")
		return false
	}
	***********/
	curActualSize := len(cp.csReactor.curCommittee.Validators) //Hack here, should use curActualCommittee
	msg := &MoveNewRoundMessage{
		CSMsgCommonHeader: ConsensusMsgCommonHeader{
			Height:    curHeight,
			Round:     curRound,
			Sender:    cp.csReactor.myPubKey,
			Timestamp: time.Now(),
			MsgType:   CONSENSUS_MSG_MOVE_NEW_ROUND,
		},

		Height:   curHeight,
		CurRound: curRound,
		NewRound: curRound + 1,
		//CurProposer: cp.csReactor.curActualCommittee[curRound].PubKey,
		//NewProposer: cp.csReactor.curActualCommittee[curRound+1].PubKey,
		CurProposer: cp.csReactor.curCommittee.Validators[curRound%curActualSize].PubKey,
		NewProposer: cp.csReactor.curCommittee.Validators[(curRound+1)%curActualSize].PubKey,
	}

	// state to init & send move to next round
	fmt.Println("msg: %v", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	fmt.Println("current state %v, move to state init", curState)
	cp.state = COMMITTEE_PROPOSER_INIT
	return true
}

// Check Kblock in available in
func (cp *ConsensusProposer) CheckKblock() bool {

	return false
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
		kblock, err := cp.buildKBlock()
		if err != nil {
			//cp.csReactor.Logger.Error("build Kblock failed ...")
			fmt.Println("build Kblock failed ...")
			return false
		}
		cp.curProposedBlock = kblock
		cp.curProposedBlockType = PROPOSE_MSG_SUBTYPE_KBLOCK

		cp.GenerateKBlockMsg(kblock)

	} else {

		mblock, err := cp.buildMBlock(proposalEmptyBlock)
		if err != nil {
			//cp.csReactor.Logger.Error("build Mblock failed ...")
			fmt.Println("build Mblock failed ...")
			return false
		}
		cp.curProposedBlock = mblock
		cp.curProposedBlockType = PROPOSE_MSG_SUBTYPE_MBLOCK

		cp.GenerateMBlockMsg(mblock)
	}

	return true
}

//XXX: ConsensusLeader always propose the 1st block. With meta data of group
func (cp *ConsensusProposer) GenerateMBlockMsg(mblock []byte) bool {
	//logger := cp.csReactor.Logger

	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     cp.csReactor.myPubKey,
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: PROPOSE_MSG_SUBTYPE_MBLOCK,
	}

	msg := &ProposalBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		CommitteeID:      cp.CommitteeID,
		ProposerID:       cp.csReactor.myPubKey,
		CSProposerPubKey: cp.csReactor.csCommon.system.PubKeyToBytes(cp.csReactor.csCommon.PubKey),
		KBlockHeight:     0, //TBD
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(mblock),
		ProposedBlock:    mblock,
	}

	fmt.Println("Proposal Message:", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	cp.state = COMMITTEE_PROPOSER_PROPOSED

	//timeout function
	proposalExpire := func() {
		fmt.Println("reach 2/3 votes of proposal expired ...")
		fmt.Println("the committeeSize", cp.csReactor.committeeSize, "the voter", cp.proposalVoterNum)
		cp.MoveInitState(cp.state)
	}
	cp.proposalThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, proposalExpire)

	return true
}

// Build MBlock for consensus and commit it after consensus
// ConsenusLeader generate the 1st block. With meta data of group
func (cp *ConsensusProposer) buildMBlock(buildEmptyBlock bool) ([]byte, error) {
	//logger := cp.csReactor.Logger

	/****************
	txs := blockchain.BuildMBlockTxs(-1)
	if buildEmptyBlock == false && len(txs) == 0 {
		//logger.Error("No txs, but buildEmptyBlock is disabled ...")
		fmt.Println("No txs, but buildEmptyBlock is disabled ...")
		return []byte{}, nil
	}

	mblock, err := blockchain.PrepareMBlock(cp.csReactor.curHeight, cp.csReactor.lastKBlockHeight,
		blockchain.BLOCKTYPE_MBLOCK, cp.csReactor.myPubKey, txs)
	if err != nil {
		//logger.Error("build Mblock failed ...")
		fmt.Println("build Mblock failed ...")
		return []byte{}, nil
	}

	block, err := mblock.Serialize()
	if err != nil {
		//logger.Error("build Mblock failed")
		fmt.Println("build Mblock failed ...")
		return []byte{}, nil
	}
	***************/
	blk := BuildMBlock()
	blkBytes := block.BlockEncodeBytes(blk)
	return blkBytes, nil
}

// Propose the Kblock
func (cp *ConsensusProposer) GenerateKBlockMsg(kblock []byte) bool {

	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     cp.csReactor.myPubKey,
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_PROPOSAL_BLOCK,
		MsgSubType: PROPOSE_MSG_SUBTYPE_KBLOCK,
	}

	msg := &ProposalBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		CommitteeID:      cp.CommitteeID,
		ProposerID:       cp.csReactor.myPubKey,
		CSProposerPubKey: cp.csReactor.csCommon.system.PubKeyToBytes(cp.csReactor.csCommon.PubKey),
		KBlockHeight:     0, //TBD
		SignOffset:       MSG_SIGN_OFFSET_DEFAULT,
		SignLength:       MSG_SIGN_LENGTH_DEFAULT,
		ProposedSize:     len(kblock),
		ProposedBlock:    kblock,
	}

	fmt.Println("Proposal Message: ", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	cp.state = COMMITTEE_PROPOSER_PROPOSED

	//timeout function
	proposalExpire := func() {
		fmt.Println("reach 2/3 votes of proposal expired ...")
		cp.MoveInitState(cp.state)
	}
	cp.proposalThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, proposalExpire)

	return true
}

// ConsenusLeader generate the 1st block. With meta data of group
func (cp *ConsensusProposer) buildKBlock() ([]byte, error) {
	/*************
	kblock := blockchain.PrepareKblock()
	block, err := kblock.Serialized()
	if err != nil {
		fmt.Println("build Kblock failed")
		return nil, err
	}
	***************/
	var block []byte
	return block, nil
}

// After blockproposal vote > 2/3, proposer generates NotaryBlock
func (cp *ConsensusProposer) GenerateNotaryBlockMsg() bool {

	curHeight := cp.csReactor.curHeight
	curRound := cp.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    cp.csReactor.myPubKey,
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_NOTARY_BLOCK,
	}

	msg := &NotaryBlockMessage{
		CSMsgCommonHeader: cmnHdr,

		ProposerID:    cp.csReactor.myPubKey,
		CommitteeID:   cp.CommitteeID,
		CommitteeSize: cp.csReactor.committeeSize,

		SignOffset:        MSG_SIGN_OFFSET_DEFAULT,
		SignLength:        MSG_SIGN_LENGTH_DEFAULT,
		VoterBitArray:     *cp.proposalVoterBitArray,
		VoterAggSignature: cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig),
	}

	fmt.Println("NotaryBlock Msg: ", msg.String())
	var m ConsensusMessage = msg
	cp.SendMsg(&m)
	cp.state = COMMITTEE_PROPOSER_NOTARYSENT

	return true
}

//
func (cp *ConsensusProposer) ProcessVoteForProposal(vote4ProposalMsg *VoteForProposalMessage, src *ConsensusPeer) bool {
	// only process Vote in state proposed
	if cp.state < COMMITTEE_PROPOSER_PROPOSED {
		fmt.Println("state machine incorrect, expected PROPOSED, actual ", cp.state)
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
	if ch.Height != cp.csReactor.curHeight {
		fmt.Println("Height mismatch!, curHeight %d, the incoming %d", cp.csReactor.curHeight, ch.Height)
		return false
	}

	if ch.Round != cp.csReactor.curRound {
		fmt.Println("Round mismatch!, curRound %d, the incoming %d", cp.csReactor.curRound, ch.Round)
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_VOTE_FOR_PROPOSAL {
		fmt.Println("MsgType is not CONSENSUS_MSG_VOTE_FOR_PROPOSAL")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	index := cp.csReactor.GetCommitteeMemberIndex(ch.Sender)
	if index != vote4ProposalMsg.VoterIndex {
		fmt.Println("Voter index mismatch %d vs %d", index, vote4ProposalMsg.VoterIndex)
		return false
	}

	//so far so good
	// 1. validate voter signature
	myPubKey := cp.csReactor.myPubKey
	signMsg := cp.csReactor.BuildProposalBlockSignMsg(myPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))
	fmt.Println("sign message: ", signMsg)

	// validate the message hash
	msgHash := cp.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(MSG_SIGN_OFFSET_DEFAULT), uint32(MSG_SIGN_LENGTH_DEFAULT))
	if msgHash != vote4ProposalMsg.SignedMessageHash {
		fmt.Println("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := cp.csReactor.csCommon.system.SigFromBytes(vote4ProposalMsg.VoterSignature)
	if err != nil {
		fmt.Println("get signature failed ...")
		return false
	}

	pubKey, err := cp.csReactor.csCommon.system.PubKeyFromBytes(vote4ProposalMsg.CSVoterPubKey)
	if err != nil {
		fmt.Println("get PubKey failed ...")
		return false
	}

	valid := bls.Verify(sig, msgHash, pubKey)
	if valid == false {
		fmt.Println("validate voter signature failed")
		return false
	}

	// 2. add src to bitArray.
	cp.proposalVoterNum++
	cp.proposalVoterBitArray.SetIndex(index, true)

	cp.proposalVoterSig = append(cp.proposalVoterSig, sig)
	cp.proposalVoterPubKey = append(cp.proposalVoterPubKey, pubKey)
	cp.notaryVoterMsgHash = append(cp.proposalVoterMsgHash, msgHash)

	// XXX::: Yang: Hack here +2 to pass 2/3
	// 3. if the totoal vote > 2/3, move to NotarySent state
	//if cp.proposalVoterNum >= cp.csReactor.committeeSize*2/3 &&
	if (cp.proposalVoterNum+1) >= cp.csReactor.committeeSize*2/3 &&
		cp.state == COMMITTEE_PROPOSER_PROPOSED {

		// Now it is OK to aggregate signatures
		cp.proposalVoterAggSig = cp.csReactor.csCommon.AggregateSign(cp.proposalVoterSig)

		//send out notary
		cp.GenerateNotaryBlockMsg()
		cp.state = COMMITTEE_PROPOSER_NOTARYSENT
		cp.proposalThresholdTimer.Stop()

		//timeout function
		notaryBlockExpire := func() {
			fmt.Println("reach 2/3 vote of notaryBlock expired ...")
			fmt.Println("received votes of notary", cp.notaryVoterNum, "committeeSize", cp.csReactor.committeeSize)
			cp.MoveInitState(cp.state)
		}

		cp.notaryThresholdTimer = time.AfterFunc(PROPOSER_THRESHOLD_TIMER_TIMEOUT, notaryBlockExpire)
	}
	return true
}

func (cp *ConsensusProposer) ProcessVoteForNotary(vote4NotaryMsg *VoteForNotaryMessage, src *ConsensusPeer) bool {
	//logger := cp.csReactor.Logger

	// only process Vote Notary in state NotarySent
	if cp.state != COMMITTEE_PROPOSER_NOTARYSENT {
		fmt.Println("state machine incorrect, expected NOTARYSENT, actual %v", cp.state)
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
	if ch.Height != cp.csReactor.curHeight {
		fmt.Println("Height mismatch!, curHeight %d, the incoming %d", cp.csReactor.curHeight, ch.Height)
		return false
	}

	if ch.Round != cp.csReactor.curRound {
		fmt.Println("Round mismatch!, curRound %d, the incoming %d", cp.csReactor.curRound, ch.Round)
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_VOTE_FOR_NOTARY {
		fmt.Println("MsgType is not CONSENSUS_MSG_VOTE_FOR_NOTARY")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	index := cp.csReactor.GetCommitteeMemberIndex(ch.Sender)
	if index != vote4NotaryMsg.VoterIndex {
		fmt.Println("Voter index mismatch %d vs %d", index, vote4NotaryMsg.VoterIndex)
		return false
	}

	//so far so good
	// 1. validate voter signature
	myPubKey := cp.csReactor.myPubKey
	signMsg := cp.csReactor.BuildNotaryBlockSignMsg(myPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))
	fmt.Println("sign message: ", signMsg)

	// validate the message hash
	msgHash := cp.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(MSG_SIGN_OFFSET_DEFAULT), uint32(MSG_SIGN_LENGTH_DEFAULT))
	if msgHash != vote4NotaryMsg.SignedMessageHash {
		fmt.Println("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := cp.csReactor.csCommon.system.SigFromBytes(vote4NotaryMsg.VoterSignature)
	if err != nil {
		fmt.Println("get signature failed ...")
		return false
	}

	pubKey, err := cp.csReactor.csCommon.system.PubKeyFromBytes(vote4NotaryMsg.CSVoterPubKey)
	if err != nil {
		fmt.Println("get PubKey failed ...")
		return false
	}

	valid := bls.Verify(sig, msgHash, pubKey)
	if valid == false {
		fmt.Println("validate voter signature failed")
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
		fmt.Println("==============================================================\n")
		fmt.Println("This block proposal is approved, commit it ... Height", cp.csReactor.curHeight, "Round", cp.csReactor.curRound)
		fmt.Println("Move to next height")
		fmt.Println("===============================================================")
		//logger.Info("")

		// only the block body are filled. Now fill the Evidence / committeeInfo/ Kblock Data if needed
		votingSig := cp.csReactor.csCommon.system.SigToBytes(cp.proposalVoterAggSig)
		notarizeSig := cp.csReactor.csCommon.system.SigToBytes(cp.notaryVoterAggSig)
		evidence := block.NewEvidence(votingSig, *cp.proposalVoterBitArray, notarizeSig, *cp.notaryVoterBitArray)

		blkBytes := cp.curProposedBlock
		blk, err := block.BlockDecodeFromBytes(blkBytes)
		if err != nil {
			fmt.Println("decode block failed")
			goto INIT_STATE
		}

		if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_KBLOCK {
			// XXX fill KBlockData later
			var kBlockData []byte
			cp.csReactor.finalizeKBlock(blk, evidence, kBlockData)
		} else if cp.curProposedBlockType == PROPOSE_MSG_SUBTYPE_KBLOCK {
			cp.csReactor.finalizeMBlock(blk, evidence)
		}

		if cp.csReactor.finalizeCommitBlock(blk) == true {
			fmt.Println("commit block successfully")
		} else {
			fmt.Println("commit block failed ...")
		}
	}

INIT_STATE:
	//Finally, go to init
	time.Sleep(5 * time.Second)
	cp.MoveInitState(cp.state)

	cp.csReactor.UpdateRound(cp.csReactor.curRound + 1)

	return true
}
