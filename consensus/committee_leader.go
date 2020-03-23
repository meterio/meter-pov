/*****
 Leader Functionalities:
	if kblock is approved by old committee,
        i)  create group announce
        ii) consensus this group announcement
        iii)first proposer of block (in this proposer.go)
***/

package consensus

import (
	"bytes"
	"fmt"
	"time"

	"github.com/dfinlab/meter/block"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	// FSM of Committee Leader
	COMMITTEE_LEADER_INIT      = byte(0x01)
	COMMITTEE_LEADER_ANNOUNCED = byte(0x02)
	// COMMITTEE_LEADER_NOTARYSENT = byte(0x03)
	COMMITTEE_LEADER_COMMITED = byte(0x04)

	THRESHOLD_TIMER_TIMEOUT = 4 * time.Second //wait for reach 2/3 consensus timeout
)

type ConsensusLeader struct {
	EpochID   uint64
	Nonce     uint64
	state     byte
	csReactor *ConsensusReactor //global reactor info
	replay    bool

	announceVoterIndexs []int

	// newCommittee voting evidence
	voterBitArray *cmn.BitArray
	voterMsgHash  [32]byte
	voterAggSig   bls.Signature

	//signature data
	// announceVoterBitArray *cmn.BitArray
	// announceVoterSig      []bls.Signature
	// announceVoterPubKey   []bls.PublicKey
	// announceVoterMsgHash  [][32]byte
	// announceVoterAggSig   bls.Signature
	// announceVoterNum      int

	announceSigAggregator *SignatureAggregator

	announceThresholdTimer *time.Timer // 2/3 voting timer
	notaryThresholdTimer   *time.Timer // notary 2/3 vote timer
}

// send consensus message to all connected peers
func (cl *ConsensusLeader) SendMsg(msg ConsensusMessage) bool {
	var peers []*ConsensusPeer
	switch msg.(type) {
	case *AnnounceCommitteeMessage:
		peers = cl.CreateAnnounceMsgPeers()

	case *NotaryAnnounceMessage:
		peers = cl.CreateNotaryMsgPeers() // TBD: makes smaller set for notary

	default:
		cl.csReactor.logger.Info("Wrong type of leader messages")
		peers = []*ConsensusPeer{}
	}
	return cl.csReactor.SendMsgToPeers(peers, &msg)
}

// Move to the init State
func (cl *ConsensusLeader) MoveInitState(curState byte) bool {
	// should not send move to next round message for leader state machine
	r := cl.csReactor
	cl.csReactor.logger.Info("Move to init state of leader",
		"curHeight", r.curHeight,
		"curState", curState,
		"actualComitteeSize", len(r.curActualCommittee),
		"comitteeSize", len(r.curCommittee.Validators))
	cl.state = COMMITTEE_LEADER_INIT
	return true
}

//New CommitteeLeader
func NewCommitteeLeader(conR *ConsensusReactor) *ConsensusLeader {
	cl := &ConsensusLeader{
		Nonce:     conR.curNonce,
		state:     COMMITTEE_LEADER_INIT,
		csReactor: conR,
		EpochID:   conR.curEpoch,
		// announceVoterBitArray: cmn.NewBitArray(conR.committeeSize),
		// notaryVoterBitArray:   cmn.NewBitArray(conR.committeeSize),
	}
	return cl
}

// curCommittee others except myself
func (cl *ConsensusLeader) CreateAnnounceMsgPeers() []*ConsensusPeer {
	csPeers := make([]*ConsensusPeer, 0)
	for i, v := range cl.csReactor.curCommittee.Validators {
		if i == cl.csReactor.curCommitteeIndex {
			continue
		}
		// initialize PeerConn
		p := newConsensusPeer(v.Name, v.NetAddr.IP, v.NetAddr.Port, cl.csReactor.magic)
		csPeers = append(csPeers, p)
	}
	return csPeers
}

// ActulCommittee except myself
func (cl *ConsensusLeader) CreateNotaryMsgPeers() []*ConsensusPeer {
	csPeers := make([]*ConsensusPeer, 0)
	for _, cm := range cl.csReactor.curActualCommittee {
		if cm.CSIndex == cl.csReactor.curCommitteeIndex {
			continue
		}
		// initialize PeerConn
		p := newConsensusPeer(cm.Name, cm.NetAddr.IP, cm.NetAddr.Port, cl.csReactor.magic)
		csPeers = append(csPeers, p)
	}
	return csPeers
}

// Committee leader create AnnounceCommittee to all peers
func (cl *ConsensusLeader) GenerateAnnounceMsg() bool {

	curHeight := cl.csReactor.curHeight

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     0,
		Sender:    crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_ANNOUNCE_COMMITTEE,
		EpochID:   cl.EpochID,
	}

	best := cl.csReactor.chain.BestBlock()
	var kblockHeight int64
	if best.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
		kblockHeight = int64(best.Header().Number())
	} else {
		// mblock
		kblockHeight = int64(best.Header().LastKBlockHeight())
	}

	msg := &AnnounceCommitteeMessage{
		CSMsgCommonHeader: cmnHdr,

		AnnouncerID:    crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		AnnouncerBlsPK: cl.csReactor.csCommon.GetSystem().PubKeyToBytes(*cl.csReactor.csCommon.GetPublicKey()),

		CommitteeSize:  cl.csReactor.committeeSize,
		Nonce:          cl.Nonce,
		KBlockHeight:   kblockHeight,
		POWBlockHeight: 0, //TODO: TBD

		// signature from newcommittee
		VotingBitArray: cl.voterBitArray,
		VotingMsgHash:  cl.voterMsgHash,
		VotingAggSig:   cl.csReactor.csCommon.GetSystem().SigToBytes(cl.voterAggSig),
	}

	// sign message with ecdsa key
	msgSig, err := cl.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cl.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cl.csReactor.logger.Debug("Generate Announce Comittee Message", "msg", msg.String())

	var m ConsensusMessage = msg
	cl.state = COMMITTEE_LEADER_ANNOUNCED
	cl.SendMsg(m)

	signMsg := cl.csReactor.BuildAnnounceSignMsg(cl.csReactor.myPubKey, cmnHdr.EpochID, uint64(cmnHdr.Height), uint32(cmnHdr.Round))
	msgHash := cl.csReactor.csCommon.Hash256Msg([]byte(signMsg))
	cl.announceSigAggregator = newSignatureAggregator(cl.csReactor.committeeSize, cl.csReactor.csCommon.system, msgHash)
	//timeout function
	announceExpire := func() {
		cl.csReactor.logger.Warn("reach 2/3 votes of announce expired ...", "comitteeSize", cl.csReactor.committeeSize, "totalComitter", cl.announceSigAggregator.Count())

		if LeaderMajorityTwoThird(int(cl.announceSigAggregator.Count()), cl.csReactor.committeeSize) && cl.state == COMMITTEE_LEADER_ANNOUNCED {

			cl.csReactor.logger.Info("Committers reach 2/3 of Committee")

			//stop announce Timer
			//cl.announceThresholdTimer.Stop()

			// Aggregate signature here
			cl.announceSigAggregator.Aggregate()
			// cl.announceVoterAggSig = cl.csReactor.csCommon.AggregateSign(cl.announceVoterSig)
			cl.csReactor.UpdateActualCommittee(cl.csReactor.curCommitteeIndex)

			//send out announce notary
			// cl.state = COMMITTEE_LEADER_NOTARYSENT
			cl.GenerateNotaryAnnounceMsg()

			//Now Committee is already announced establishment. Wait a little bit while of message transimit
			notaryExpire := func() {
				cl.csReactor.logger.Info("NotaryAnnounce sent", "comitteeSize", cl.csReactor.committeeSize)
				cl.committeeEstablished()
			}
			cl.notaryThresholdTimer = time.AfterFunc(1*time.Second, func() {
				cl.csReactor.schedulerQueue <- notaryExpire
			})

		} else {
			cl.csReactor.logger.Warn("did not reach 2/3 committer of announce ...", "comitteeSize", cl.csReactor.committeeSize, "totalComitter", cl.announceSigAggregator.Count())
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

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     0,
		Sender:    crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_NOTARY_ANNOUNCE,
		EpochID:   cl.EpochID,
	}

	for _, cm := range cl.csReactor.curActualCommittee {
		fmt.Println("CUR CM: ", cm)
	}
	msg := &NotaryAnnounceMessage{
		CSMsgCommonHeader: cmnHdr,

		AnnouncerID:    crypto.FromECDSAPub(&cl.csReactor.myPubKey),
		AnnouncerBlsPK: cl.csReactor.csCommon.GetSystem().PubKeyToBytes(*cl.csReactor.csCommon.GetPublicKey()),

		VotingBitArray: cl.voterBitArray,
		VotingMsgHash:  cl.voterMsgHash,
		VotingAggSig:   cl.csReactor.csCommon.GetSystem().SigToBytes(cl.voterAggSig),

		NotarizeBitArray: cl.announceSigAggregator.bitArray,
		NotarizeMsgHash:  cl.announceSigAggregator.msgHash,
		NotarizeAggSig:   cl.announceSigAggregator.Aggregate(),

		CommitteeSize:    cl.csReactor.committeeSize,
		CommitteeMembers: cl.csReactor.BuildCommitteeInfoFromMember(cl.csReactor.csCommon.GetSystem(), cl.csReactor.curActualCommittee),
	}

	// sign message with ecdsa key
	msgSig, err := cl.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cl.csReactor.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cl.csReactor.logger.Debug("Generate Notary Announce Message", "msg", msg.String())

	var m ConsensusMessage = msg
	cl.SendMsg(m)
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

	ch := commitMsg.CSMsgCommonHeader
	if ch.Height != cl.csReactor.curHeight {
		cl.csReactor.logger.Error("Height mismatch!", "curHeight", cl.csReactor.curHeight, "incomingHeight", ch.Height)
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

	validator := cl.csReactor.curCommittee.Validators[index]
	if bytes.Equal(crypto.FromECDSAPub(&validator.PubKey), commitMsg.CommitterID) == false {
		cl.csReactor.logger.Error("ecdsa public key mismatch", "index", index)
		return false
	}
	if bytes.Equal(cl.csReactor.csCommon.GetSystem().PubKeyToBytes(validator.BlsPubKey), commitMsg.CommitterBlsPK) == false {
		cl.csReactor.logger.Error("bls public key mismatch", "index", index)
		return false
	}

	//so far so good
	// 1. validate vote signature
	myPubKey := cl.csReactor.myPubKey
	signMsg := cl.csReactor.BuildAnnounceSignMsg(myPubKey, commitMsg.CSMsgCommonHeader.EpochID, uint64(ch.Height), uint32(ch.Round))
	cl.csReactor.logger.Debug("Sign message", "msg", signMsg)

	// validate the message hash
	msgHash := cl.csReactor.csCommon.Hash256Msg([]byte(signMsg))
	if msgHash != commitMsg.SignedMsgHash {
		cl.csReactor.logger.Error("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := cl.csReactor.csCommon.GetSystem().SigFromBytes(commitMsg.BlsSignature)
	if err != nil {
		cl.csReactor.logger.Error("get signature failed ...")
		return false
	}

	valid := bls.Verify(sig, msgHash, validator.BlsPubKey)
	if valid == false {
		cl.csReactor.logger.Error("validate voter signature failed")
		if cl.csReactor.config.SkipSignatureCheck == true {
			cl.csReactor.logger.Error("but SkipSignatureCheck is true, continue ...")
		} else {
			return false
		}
	}

	// 2. add src to bitArray.
	cl.announceSigAggregator.Add(index, msgHash, commitMsg.BlsSignature, validator.BlsPubKey)
	return true
}

func (cl *ConsensusLeader) committeeEstablished() error {
	cl.state = COMMITTEE_LEADER_COMMITED
	cl.notaryThresholdTimer.Stop()

	//aggregate signature
	// Aggregate signature here
	// cl.notaryVoterAggSig = cl.csReactor.csCommon.AggregateSign(cl.notaryVoterSig)

	//Finally, go to init
	cl.MoveInitState(cl.state)

	//Committee is established. Myself is Leader, server as 1st proposer.
	cl.csReactor.logger.Info(`
===========================================================
Committee is established!!! ...
Myself is Leader, Let's start the pacemaker.
===========================================================`, "Committee Epoch", cl.EpochID)

	//Now we are in new epoch
	cl.csReactor.updateCurEpoch(cl.EpochID)

	//Now move to propose the 1st block in round 0
	cl.csReactor.enterConsensusValidator()
	cl.csReactor.csValidator.state = COMMITTEE_VALIDATOR_COMMITSENT

	// clean up
	cl.csReactor.NewCommitteeCleanup()

	// Now start the pacemaker
	newCommittee := !cl.replay
	cl.csReactor.startPacemaker(newCommittee)
	return nil
}
