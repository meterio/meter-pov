/*****
 Leader Functionalities:
	if kblock is approved by old committee,
        i)  create group announce
        ii) consensus this group announcement
        iii)first proposer of block (in this proposer.go)
***/

package consensus

import (
	//    "errors"

	"fmt"
	"time"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	// FSM of Committee Leader
	COMMITTEE_LEADER_INIT       = byte(0x01)
	COMMITTEE_LEADER_ANNOUNCED  = byte(0x02)
	COMMITTEE_LEADER_NOTARYSENT = byte(0x03)
	COMMITTEE_LEADER_COMMITED   = byte(0x04)

	THRESHOLD_TIMER_TIMEOUT = 3 * time.Second //wait for reach 2/3 consensus timeout
	// 1s by default
)

type ConsensusLeader struct {
	node_id      uint32
	consensus_id uint32 // unique identifier for this consensus session

	EpochID   uint64
	Nonce     uint64
	state     byte
	csReactor *ConsensusReactor //global reactor info
	replay    bool

	//signature data
	announceVoterBitArray *cmn.BitArray
	announceVoterIndexs   []int
	announceVoterSig      []bls.Signature
	announceVoterPubKey   []bls.PublicKey
	announceVoterMsgHash  [][32]byte
	announceVoterAggSig   bls.Signature
	announceVoterNum      int

	//
	notaryVoterBitArray *cmn.BitArray
	notaryVoterIndexes  []int
	notaryVoterSig      []bls.Signature
	notaryVoterPubKey   []bls.PublicKey
	notaryVoterMsgHash  [][32]byte
	notaryVoterAggSig   bls.Signature
	notaryVoterNum      int

	announceThresholdTimer *time.Timer // 2/3 voting timer
	notaryThresholdTimer   *time.Timer // notary 2/3 vote timer

	csPeers []*ConsensusPeer // consensus message peers
}

// send consensus message to all connected peers
func (cl *ConsensusLeader) SendMsg(msg *ConsensusMessage) bool {
	return cl.csReactor.SendMsgToPeers(cl.csPeers, msg)
}

// Move to the init State
func (cl *ConsensusLeader) MoveInitState(curState byte) bool {
	// should not send move to next round message for leader state machine
	r := cl.csReactor
	cl.csReactor.logger.Info("Move to init state of leader",
		"curHeight", r.curHeight, "curRound", r.curRound,
		"curState", curState,
		"actualComitteeSize", len(r.curActualCommittee),
		"comitteeSize", len(r.curCommittee.Validators))
	cl.state = COMMITTEE_LEADER_INIT
	return true
}

//New CommitteeLeader
func NewCommitteeLeader(conR *ConsensusReactor) *ConsensusLeader {
	cl := &ConsensusLeader{
		Nonce:                 conR.curNonce,
		state:                 COMMITTEE_LEADER_INIT,
		csReactor:             conR,
		EpochID:               conR.curEpoch,
		announceVoterBitArray: cmn.NewBitArray(conR.committeeSize),
		notaryVoterBitArray:   cmn.NewBitArray(conR.committeeSize),
	}

	// form topology, we know the 0 is Leader itself
	fmt.Println(conR.curCommittee)
	for _, v := range conR.curCommittee.Validators[1:] {
		// initialize PeerConn
		p := newConsensusPeer(v.NetAddr.IP, v.NetAddr.Port)
		cl.csPeers = append(cl.csPeers, p)
	}
	return cl
}

// Committee leader create AnnounceCommittee to all peers
func (cl *ConsensusLeader) GenerateAnnounceMsg() bool {

	curHeight := cl.csReactor.curHeight
	curRound := cl.csReactor.curRound

	// curRound must be zero while sending announce
	if curRound != 0 {
		cl.csReactor.logger.Error("curRound must be zero while sending announce", "expected", 0, "actual", curRound)
		curRound = 0
	}

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_ANNOUNCE_COMMITTEE,
		EpochID:   cl.EpochID,
	}

	paramBytes, _ := cl.csReactor.csCommon.params.ToBytes()

	msg := &AnnounceCommitteeMessage{
		CSMsgCommonHeader: cmnHdr,

		AnnouncerID:   crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		CommitteeSize: cl.csReactor.committeeSize,
		Nonce:         cl.Nonce,

		CSParams:       paramBytes,
		CSSystem:       cl.csReactor.csCommon.system.ToBytes(),
		CSLeaderPubKey: cl.csReactor.csCommon.system.PubKeyToBytes(cl.csReactor.csCommon.PubKey),
		KBlockHeight:   0, //TBD, last Kblock Height
		POWBlockHeight: 0, //TBD

		SignOffset: MSG_SIGN_OFFSET_DEFAULT,
		SignLength: MSG_SIGN_LENGTH_DEFAULT,
	}

	// sign message
	msgSig, err := cl.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cl.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cl.csReactor.logger.Debug("Generate Announce Comittee Message", "msg", msg.String())

	var m ConsensusMessage = msg
	cl.state = COMMITTEE_LEADER_ANNOUNCED
	cl.SendMsg(&m)

	//timeout function
	announceExpire := func() {
		cl.csReactor.logger.Warn("reach 2/3 votes of announce expired ...", "comitteeSize", cl.csReactor.committeeSize, "totalComitter", cl.announceVoterNum)

		if LeaderMajorityTwoThird(cl.announceVoterNum, cl.csReactor.committeeSize) && cl.state == COMMITTEE_LEADER_ANNOUNCED {

			cl.csReactor.logger.Info("Committers reach 2/3 of Committee")

			//stop announce Timer
			//cl.announceThresholdTimer.Stop()

			// Aggregate signature here
			cl.announceVoterAggSig = cl.csReactor.csCommon.AggregateSign(cl.announceVoterSig)
			cl.csReactor.UpdateActualCommittee(cl.announceVoterIndexs, cl.announceVoterPubKey, cl.announceVoterBitArray)

			//send out announce notary
			cl.state = COMMITTEE_LEADER_NOTARYSENT
			cl.GenerateNotaryAnnounceMsg()

			//timeout function
			notaryExpire := func() {
				cl.csReactor.logger.Warn("reach 2/3 votes of notary expired ...", "comitteeSize", cl.csReactor.committeeSize, "totalComitter", cl.announceVoterNum)
				cl.MoveInitState(cl.state)
			}
			cl.notaryThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, func() {
				cl.csReactor.schedulerQueue <- notaryExpire
			})

		} else {
			cl.csReactor.logger.Warn("did not reach 2/3 committer of announce ...", "comitteeSize", cl.csReactor.committeeSize, "totalComitter", cl.announceVoterNum)
			cl.MoveInitState(cl.state)
		}
	}
	cl.announceThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, func() {
		cl.csReactor.schedulerQueue <- announceExpire
	})

	return true
}

// After announce vote > 2/3, Leader generate Notary
// Committee leader create NotaryAnnounce to all members
func (cl *ConsensusLeader) GenerateNotaryAnnounceMsg() bool {

	curHeight := cl.csReactor.curHeight
	curRound := cl.csReactor.curRound

	// curRound must be zero while sending announce
	if curRound != 0 {
		cl.csReactor.logger.Error("curRound must be zero while sending announce", "expected", 0, "actual", curRound)
		curRound = 0
	}

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_NOTARY_ANNOUNCE,
		EpochID:   cl.EpochID,
	}

	msg := &NotaryAnnounceMessage{
		CSMsgCommonHeader: cmnHdr,

		AnnouncerID:   crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		CommitteeSize: cl.csReactor.committeeSize,

		SignOffset:             MSG_SIGN_OFFSET_DEFAULT,
		SignLength:             MSG_SIGN_LENGTH_DEFAULT, //uint(unsafe.Sizeof(cmnHdr))
		VoterBitArray:          *cl.announceVoterBitArray,
		VoterAggSignature:      cl.csReactor.csCommon.system.SigToBytes(cl.announceVoterAggSig),
		CommitteeActualSize:    len(cl.csReactor.curActualCommittee),
		CommitteeActualMembers: cl.csReactor.BuildCommitteeInfoFromMember(cl.csReactor.csCommon.system, cl.csReactor.curActualCommittee),
	}

	// sign message
	msgSig, err := cl.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cl.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cl.csReactor.logger.Debug("Generate Notary Announce Message", "msg", msg.String())

	var m ConsensusMessage = msg
	cl.SendMsg(&m)
	// cl.state = COMMITTEE_LEADER_NOTARYSENT

	return true
}

// process commitCommittee in response of announce committee
func (cl *ConsensusLeader) ProcessCommitMsg(commitMsg *CommitCommitteeMessage, src *ConsensusPeer) bool {

	// only process Vote in state announced
	if cl.state < COMMITTEE_LEADER_ANNOUNCED {
		cl.csReactor.logger.Error("state machine incorrect", "expected", "ANNOUNCED", "actual", cl.state)
		return false
	}

	// valid the common header first
	/***
	commitMsg, ok := interface{}(commit).(CommitCommitteeMessage)
	if ok != false {
		cl.csReactor.logger.Error("Message type is not CommitCommitteeMessage")
		return false
	}
	***/

	ch := commitMsg.CSMsgCommonHeader
	if !cl.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_COMMIT_COMMITTEE {
		cl.csReactor.logger.Error("MsgType mismatch", "expected", "CONSENSUS_MSG_COMMIT_COMMITTEE", "actual", ch.MsgType)
		// cl.csReactor.logger.Error(ch)
		return false
	}

	if cl.csReactor.ValidateCMheaderSig(&ch, commitMsg.SigningHash().Bytes()) == false {
		cl.csReactor.logger.Error("Signature validate failed")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	senderPubKey, err := crypto.UnmarshalPubkey(ch.Sender)
	if err != nil {
		cl.csReactor.logger.Error("ummarshal public key of sender failed ")
		return false
	}
	index := cl.csReactor.GetCommitteeMemberIndex(*senderPubKey)
	if index != commitMsg.CommitterIndex {
		cl.csReactor.logger.Error("Voter index mismatch", "expected", index, "actual", commitMsg.CommitterIndex)
		return false
	}

	//so far so good
	// 1. validate vote signature
	myPubKey := cl.csReactor.myPubKey
	signMsg := cl.csReactor.BuildAnnounceSignMsg(myPubKey, commitMsg.CSMsgCommonHeader.EpochID, uint64(ch.Height), uint32(ch.Round))
	cl.csReactor.logger.Debug("Sign message", "msg", signMsg)

	// validate the message hash
	msgHash := cl.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(MSG_SIGN_OFFSET_DEFAULT), uint32(MSG_SIGN_LENGTH_DEFAULT))
	if msgHash != commitMsg.SignedMessageHash {
		cl.csReactor.logger.Error("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := cl.csReactor.csCommon.system.SigFromBytes(commitMsg.CommitterSignature)
	if err != nil {
		cl.csReactor.logger.Error("get signature failed ...")
		return false
	}

	pubKey, err := cl.csReactor.csCommon.system.PubKeyFromBytes(commitMsg.CSCommitterPubKey)
	if err != nil {
		cl.csReactor.logger.Error("get PubKey failed ...")
		return false
	}

	//valid := bls.Verify(commitMsg.CommitterSignature, msgHash, commitMsg.CSCommitterPubKey)
	valid := bls.Verify(sig, msgHash, pubKey)
	if valid == false {
		cl.csReactor.logger.Error("validate voter signature failed")
		if cl.csReactor.config.SkipSignatureCheck == true {
			cl.csReactor.logger.Error("but SkipSignatureCheck is true, continue ...")
		} else {
			return false
		}
	}

	// 2. add src to bitArray.
	cl.announceVoterNum++
	cl.announceVoterBitArray.SetIndex(index, true)

	// Basic we get the actual committee here, but publish in notary
	cl.announceVoterIndexs = append(cl.announceVoterIndexs, commitMsg.CommitterIndex)
	//cl.announceVoterSig = append(cl.announceVoterSig, commitMsg.CommitterSignature)
	cl.announceVoterSig = append(cl.announceVoterSig, sig)
	//cl.announceVoterPubKey = append(cl.announceVoterPubKey, commitMsg.CSCommitterPubKey)
	cl.announceVoterPubKey = append(cl.announceVoterPubKey, pubKey)
	cl.announceVoterMsgHash = append(cl.announceVoterMsgHash, commitMsg.SignedMessageHash)

	/**** Announce/Commit is special because we want receive the commit as many as possible. Move the 2/3 action to timer expire func
		// 3. if the totoal vote > 2/3, move to NotarySend state
		if cl.announceVoterNum >= (cl.csReactor.committeeSize*2/3) &&
			cl.state == COMMITTEE_LEADER_ANNOUNCED {
			//stop announce Timer
			cl.announceThresholdTimer.Stop()

			//send out notary
			cl.state = COMMITTEE_LEADER_NOTARYSENT

			//timeout function
			notaryExpire := func() {
				cl.csReactor.logger.Warn("reach 2/3 vote of notary expired ...")
				cl.MoveInitState(cl.state)
			}
			cl.notaryThresholdTimer = time.AfterFunc(THRESHOLD_TIMER_TIMEOUT, notaryExpire)
		}
	****/
	return true
}

// VoteForNotaryMessage MsgSubType is for announce is checked in validator
func (cl *ConsensusLeader) ProcessVoteNotaryAnnounce(vote4NotaryMsg *VoteForNotaryMessage, src *ConsensusPeer) bool {

	// only process Vote Notary in state NotarySent
	if cl.state != COMMITTEE_LEADER_NOTARYSENT {
		cl.csReactor.logger.Error("state machine incorrect", "expected", "COMMITTEE_LEADER_NOTARYSENT", "actual", cl.state)
		return false
	}

	// valid the common header first
	/****
	vote4NotaryMsg, ok := interface{}(vote).(VoteForNotaryMessage)
	if ok != false {
		cl.csReactor.logger.Error("Message type is not VoteForNotaryMessage")
		return false
	}
	****/

	ch := vote4NotaryMsg.CSMsgCommonHeader
	if !cl.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_VOTE_FOR_NOTARY {
		cl.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_VOTE_FOR_NOTARY")
		return false
	}

	if cl.csReactor.ValidateCMheaderSig(&ch, vote4NotaryMsg.SigningHash().Bytes()) == false {
		cl.csReactor.logger.Error("Signature validate failed")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	senderPubKey, err := crypto.UnmarshalPubkey(ch.Sender)
	if err != nil {
		cl.csReactor.logger.Error("ummarshal public key of sender failed ")
		return false
	}
	index := cl.csReactor.GetCommitteeMemberIndex(*senderPubKey)
	if index != vote4NotaryMsg.VoterIndex {
		cl.csReactor.logger.Error("Voter index mismatch", "expected", index, "actual", vote4NotaryMsg.VoterIndex)
		return false
	}

	//so far so good
	// 1. validate voter signature
	myPubKey := cl.csReactor.myPubKey
	signMsg := cl.csReactor.BuildNotaryAnnounceSignMsg(myPubKey, cl.EpochID, uint64(ch.Height), uint32(ch.Round))
	cl.csReactor.logger.Debug("Sign message", "msg", signMsg)

	// validate the message hash
	msgHash := cl.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(MSG_SIGN_OFFSET_DEFAULT), uint32(MSG_SIGN_LENGTH_DEFAULT))
	if msgHash != vote4NotaryMsg.SignedMessageHash {
		cl.csReactor.logger.Error("msgHash mismatch ...")
		return false
	}

	sig, err := cl.csReactor.csCommon.system.SigFromBytes(vote4NotaryMsg.VoterSignature)
	if err != nil {
		cl.csReactor.logger.Error("get signature failed ...")
		return false
	}

	pubKey, err := cl.csReactor.csCommon.system.PubKeyFromBytes(vote4NotaryMsg.CSVoterPubKey)
	if err != nil {
		cl.csReactor.logger.Error("get PubKey failed ...")
		return false
	}

	valid := bls.Verify(sig, msgHash, pubKey)
	if valid == false {
		cl.csReactor.logger.Error("validate voter signature failed")
		if cl.csReactor.config.SkipSignatureCheck == true {
			cl.csReactor.logger.Error("but SkipSignatureCheck is true, continue ...")
		} else {
			return false
		}
	}

	// 2. add src to bitArray.
	cl.notaryVoterNum++
	cl.notaryVoterBitArray.SetIndex(index, true)

	cl.notaryVoterIndexes = append(cl.notaryVoterIndexes, vote4NotaryMsg.VoterIndex)
	cl.notaryVoterSig = append(cl.notaryVoterSig, sig)
	cl.notaryVoterPubKey = append(cl.notaryVoterPubKey, pubKey)
	cl.notaryVoterMsgHash = append(cl.notaryVoterMsgHash, msgHash)

	// 3. if the totoal vote > 2/3, move to Commit state
	if LeaderMajorityTwoThird(cl.notaryVoterNum, cl.csReactor.committeeSize) &&
		cl.state == COMMITTEE_LEADER_NOTARYSENT {
		//save all group info as meta data
		cl.state = COMMITTEE_LEADER_COMMITED
		cl.notaryThresholdTimer.Stop()

		//aggregate signature
		// Aggregate signature here
		cl.notaryVoterAggSig = cl.csReactor.csCommon.AggregateSign(cl.notaryVoterSig)

		//Finally, go to init
		cl.MoveInitState(cl.state)

		//Committee is established. Myself is Leader, server as 1st proposer.
		cl.csReactor.logger.Info(`
===========================================================
Committee is established!!! ...
Myself is Leader, Let's start the pacemaker.
===========================================================`, "Committee Epoch", cl.EpochID)

		//Now we are in new epoch
		cl.csReactor.curEpoch = cl.EpochID

		//Now move to propose the 1st block in round 0
		cl.csReactor.enterConsensusValidator()
		cl.csReactor.csValidator.state = COMMITTEE_VALIDATOR_COMMITSENT

		// clean up
		cl.csReactor.NewCommitteeCleanup()

		// Now start the pacemaker
		newCommittee := !cl.replay
		cl.csReactor.csPacemaker.Start(cl.csReactor.chain.BestQC(), newCommittee)
		return true

	} else {
		// not reach 2/3 yet, wait for more
		cl.csReactor.logger.Debug("Vote for NotaryAnnounce processed (2/3 not reached yet, wait for more)")
		return true
	}
}

func (cl *ConsensusLeader) checkHeightAndRound(ch ConsensusMsgCommonHeader) bool {
	if ch.Height != cl.csReactor.curHeight {
		cl.csReactor.logger.Error("Height mismatch!", "curHeight", cl.csReactor.curHeight, "incomingHeight", ch.Height)
		return false
	}

	if ch.Round != cl.csReactor.curRound {
		cl.csReactor.logger.Error("Round mismatch!", "curRound", cl.csReactor.curRound, "incomingRound", ch.Round)
		return false
	}
	return true
}
