/*****
 Validator is the normal member of committee. they vote for proposal and notary.
    Functionalities include:
    1) add peer to proposer each round
    2) validate and vote for proposal and notary

    3) reponse group announce
    4) reponse shift new round
***/

package consensus

import (
	//    "errors"
	"bytes"
	"encoding/binary"

	// "fmt"
	"time"

	"github.com/dfinlab/meter/block"
	//"github.com/dfinlab/meter/chain"
	//"github.com/dfinlab/meter/runtime"
	//"github.com/dfinlab/meter/state"
	//"github.com/dfinlab/meter/tx"
	//"github.com/dfinlab/meter/xenv"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	crypto "github.com/ethereum/go-ethereum/crypto"

	types "github.com/dfinlab/meter/types"
)

// for all committee mermbers
const (
	// FSM of VALIDATOR
	COMMITTEE_VALIDATOR_INIT       = byte(0x01)
	COMMITTEE_VALIDATOR_COMMITSENT = byte(0x02)

	//MsgSubType of VoteForNotary is the responses
	VOTE_FOR_NOTARY_ANNOUNCE = byte(0x01)
	VOTE_FOR_NOTARY_BLOCK    = byte(0x02)

	NEW_ROUND_EXPECT_TIMEOUT = 150 * time.Second // 150s timeout between notary and NRM
)

var (
	expectedTimer *time.Timer
	expectedState bool
	expectedRound int
)

type ConsensusValidator struct {
	replay      bool
	CommitteeID uint32 // epoch ID of this committee

	csReactor *ConsensusReactor //global reactor info
	state     byte
	csPeers   []*ConsensusPeer // consensus message peers
}

// send consensus message to all connected peers
func (cv *ConsensusValidator) SendMsg(msg *ConsensusMessage) bool {
	return cv.csReactor.SendMsgToPeers(cv.csPeers, msg)
}

func (cv *ConsensusValidator) SendMsgToPeer(msg *ConsensusMessage, netAddr types.NetAddress) bool {
	csPeer := newConsensusPeer(netAddr.IP, netAddr.Port)
	return cv.csReactor.SendMsgToPeers([]*ConsensusPeer{csPeer}, msg)
}

//validator receives the initiated messages
func NewConsensusValidator(conR *ConsensusReactor) *ConsensusValidator {
	var cv ConsensusValidator

	// initialize the ConsenusLeader
	//cv.CommitteeID = conR.CommitteeID
	cv.state = COMMITTEE_VALIDATOR_INIT
	cv.csReactor = conR

	return &cv
}

// validator need to build consensus peer topology
func (cv *ConsensusValidator) RemoveAllcsPeers() bool {
	cv.csPeers = []*ConsensusPeer{}
	return true
}

func (cv *ConsensusValidator) AddcsPeer(netAddr types.NetAddress) bool {
	csPeer := newConsensusPeer(netAddr.IP, netAddr.Port)
	cv.csPeers = append(cv.csPeers, csPeer)
	return true
}

func (cv *ConsensusValidator) nextRoundExpectationCancel() {
	expectedState = false
	expectedRound = 0
	if expectedTimer != nil {
		expectedTimer.Stop()
		expectedTimer = nil
	}
}

func (cv *ConsensusValidator) nextRoundExpectationExpire() {
	cv.csReactor.logger.Warn("next round expecation timer expired", "round=", expectedRound)

	// now expectedRound moves to next
	cv.csReactor.UpdateRound(expectedRound) //pace myself round first

	expectedProposer := cv.csReactor.getRoundProposer(expectedRound)

	if bytes.Equal(crypto.FromECDSAPub(&expectedProposer.PubKey), crypto.FromECDSAPub(&cv.csReactor.myPubKey)) == true {
		cv.csReactor.logger.Info("*** NEXT ROUND EXPECATION EXPIRED! SET MYSELF THE PROPOSER! ***", "round", expectedRound)
		// wait longer here to guranttee whole committee in same state
		cv.csReactor.ScheduleProposer(5 * PROPOSER_THRESHOLD_TIMER_TIMEOUT)
		return
	}

	//set expectation
	expectedRound += 1
	cv.nextRoundExpectationStart(expectedRound, NEW_ROUND_EXPECT_TIMEOUT)
}

func (cv *ConsensusValidator) nextRoundExpectationStart(round int, duration time.Duration) {
	expectedState = true
	expectedRound = round
	//expectedProposer := cv.csReactor.getRoundProposer(round)
	if expectedTimer != nil {
		expectedTimer.Stop()
	}

	expectedTimer = time.AfterFunc(duration, func() {
		cv.csReactor.schedulerQueue <- cv.nextRoundExpectationExpire
	})
	cv.csReactor.logger.Info("set next round expectation", "round=", expectedRound, "timeout=", duration)
}

func (cv *ConsensusValidator) nextRoundExpectationGetRound() int {
	if expectedState {
		return expectedRound
	}
	return 0
}

// Generate commitCommittee Message
func (cv *ConsensusValidator) GenerateCommitMessage(sig bls.Signature, msgHash [32]byte) *CommitCommitteeMessage {

	curHeight := cv.csReactor.curHeight
	curRound := cv.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_COMMIT_COMMITTEE,
	}

	index := cv.csReactor.GetCommitteeMemberIndex(cv.csReactor.myPubKey)
	msg := &CommitCommitteeMessage{
		CSMsgCommonHeader: cmnHdr,

		CommitteeID:        uint32(cv.CommitteeID),
		CommitteeSize:      cv.csReactor.committeeSize,
		CommitterID:        crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		CSCommitterPubKey:  cv.csReactor.csCommon.system.PubKeyToBytes(cv.csReactor.csCommon.PubKey), //bls pubkey
		CommitterSignature: cv.csReactor.csCommon.system.SigToBytes(sig),                             //TBD
		CommitterIndex:     index,
		SignedMessageHash:  msgHash,
	}

	// sign message
	msgSig, err := cv.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cv.csReactor.logger.Error("Sign message failed", "error", err)
		return nil
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cv.csReactor.logger.Debug("Generate Commit Committee Message", "msg", msg.String())
	return msg
}

// get the proposer in validator set based on round
func (cv *ConsensusValidator) GetCurRoundProposer() *CommitteeMember {
	curRound := cv.csReactor.curRound
	return &cv.csReactor.curActualCommittee[curRound]
}

// process Announcement from committee leader, join the committee
// if I am included
func (cv *ConsensusValidator) ProcessAnnounceCommittee(announceMsg *AnnounceCommitteeMessage, src *ConsensusPeer) bool {
	//logger := cv.csReactor.Logger

	// only process this message at the state of init
	if cv.state != COMMITTEE_VALIDATOR_INIT {
		cv.csReactor.logger.Error("only process announcement in state", "expected", "COMMITTEE_VALIDATOR_INIT", "actual",
			cv.state)
		return true
	}

	//Decode Nonce form message, create validator set

	// valid the common header first
	/*** keep it for a while
	announceMsg, ok := interface{}(announce).(AnnounceCommitteeMessage)
	if ok != false {
		cv.csReactor.logger.Error("Message type is not AnnounceCommitteeMessage")
		return false
	}
	***/

	ch := announceMsg.CSMsgCommonHeader
	if !cv.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_ANNOUNCE_COMMITTEE {
		cv.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_ANNOUNCE_COMMITTEE")
		return false
	}

	if cv.csReactor.ValidateCMheaderSig(&ch, announceMsg.SigningHash().Bytes()) == false {
		cv.csReactor.logger.Error("Signature validate failed")
		return false
	}

	// valid the senderindex is leader from the publicKey
	if bytes.Equal(ch.Sender, announceMsg.AnnouncerID) == false {
		cv.csReactor.logger.Error("Announce sender and AnnouncerID mismatch")
		return false
	}

	// Now the announce message is OK

	//get the nonce
	nonce := announceMsg.Nonce
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)
	role, inCommittee := cv.csReactor.NewValidatorSetByNonce(buf)
	if !inCommittee {
		cv.csReactor.logger.Error("I am not in committee, do nothing ...")
		return false
	}

	if (role != CONSENSUS_COMMIT_ROLE_VALIDATOR) && (role != CONSENSUS_COMMIT_ROLE_LEADER) {
		cv.csReactor.logger.Error("I am not the validtor/leader of committee ...")
		return false
	}

	// Verify Leader is announce sender?
	lv := cv.csReactor.curCommittee.Validators[0]
	if bytes.Equal(crypto.FromECDSAPub(&lv.PubKey), ch.Sender) == false {
		cv.csReactor.logger.Error("Sender is not leader in my committee ...")
		return false
	}

	// update cspeers, build consensus peer topology
	// Right now is HUB topology, simply point back to proposer or leader
	cv.RemoveAllcsPeers()
	// cv.AddcsPeer(src.netAddr)

	// I am in committee, sends the commit message to join the CommitCommitteeMessage
	//build signature
	// initiate csCommon based on received params and system
	if cv.csReactor.csCommon != nil {
		cv.csReactor.csCommon.ConsensusCommonDeinit()
		cv.csReactor.csCommon = nil
	}

	if cv.replay {
		cv.csReactor.csCommon = NewValidatorReplayConsensusCommon(cv.csReactor, announceMsg.CSParams, announceMsg.CSSystem)
		cv.replay = false
	} else {
		cv.csReactor.csCommon = NewValidatorConsensusCommon(cv.csReactor, announceMsg.CSParams, announceMsg.CSSystem)
	}

	offset := announceMsg.SignOffset
	length := announceMsg.SignLength

	announcerPubKey, err := crypto.UnmarshalPubkey(announceMsg.AnnouncerID)
	if err != nil {
		cv.csReactor.logger.Error("ummarshal announcer public key of sender failed ")
		return false
	}
	signMsg := cv.csReactor.BuildAnnounceSignMsg(*announcerPubKey, uint32(announceMsg.CommitteeID), uint64(ch.Height), uint32(ch.Round))
	// fmt.Println("offset & length: ", offset, length, "sign msg:", signMsg)

	if int(offset+length) > len(signMsg) {
		cv.csReactor.logger.Error("out of boundary ...")
		return false
	}

	cv.CommitteeID = announceMsg.CommitteeID

	sign := cv.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := cv.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))
	msg := cv.GenerateCommitMessage(sign, msgHash)

	var m ConsensusMessage = msg
	leaderNetAddr := src.netAddr
	cv.SendMsgToPeer(&m, leaderNetAddr)
	cv.state = COMMITTEE_VALIDATOR_COMMITSENT

	//update conR
	cv.csReactor.curRound = 0
	cv.csReactor.curEpoch = cv.CommitteeID
	cv.csReactor.logger.Info("curEpoch is updated", "curEpoch=", cv.CommitteeID)
	return true
}

// Generate VoteForProposal Message
func (cv *ConsensusValidator) GenerateVoteForProposalMessage(sig bls.Signature, msgHash [32]byte) *VoteForProposalMessage {

	curHeight := cv.csReactor.curHeight
	curRound := cv.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     curRound,
		Sender:    crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_VOTE_FOR_PROPOSAL,
	}

	index := cv.csReactor.GetCommitteeMemberIndex(cv.csReactor.myPubKey)
	msg := &VoteForProposalMessage{
		CSMsgCommonHeader: cmnHdr,

		VoterID:           crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		CSVoterPubKey:     cv.csReactor.csCommon.system.PubKeyToBytes(cv.csReactor.csCommon.PubKey),
		VoterSignature:    cv.csReactor.csCommon.system.SigToBytes(sig), //TBD
		VoterIndex:        index,
		SignedMessageHash: msgHash,
	}

	// sign message
	msgSig, err := cv.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cv.csReactor.logger.Error("Sign message failed", "error", err)
		return nil
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cv.csReactor.logger.Debug("Generate Voter For Proposal Message", "msg", msg.String())
	return msg
}

func (cv *ConsensusValidator) ProcessProposalBlockMessage(proposalMsg *ProposalBlockMessage, src *ConsensusPeer) bool {
	//logger := cv.csReactor.Logger

	// only process this message at the state of commitsent
	if cv.state != COMMITTEE_VALIDATOR_COMMITSENT {
		cv.csReactor.logger.Error("only process announcement in state", "expected", "COMMITTEE_VALIDATOR_COMMITSENT", "actual",
			cv.state)
		return false
	}

	// valid the common header first
	/****
	proposalMsg, ok := interface{}(proposal).(ProposalBlockMessage)
	if ok != false {
		fmt.Println("Message type is not ProposalBlockMessage")
		return false
	}
	****/

	ch := proposalMsg.CSMsgCommonHeader
	if !cv.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_PROPOSAL_BLOCK {
		cv.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_PROPOSAL_BLOCK")
		return false
	}

	// valid the senderindex is leader from the publicKey
	if bytes.Equal(ch.Sender, proposalMsg.ProposerID) == false {
		cv.csReactor.logger.Error("Proposal sender and proposalID mismatch")
		return false
	}

	// Now the proposal message is OK
	blkBytes := proposalMsg.ProposedBlock
	size := proposalMsg.ProposedSize
	if size != len(blkBytes) {
		//logger.Error("proposal block size mismatch")
		cv.csReactor.logger.Error("proposal block size mismatch ...")
		return false
	}

	blk, err := block.BlockDecodeFromBytes(blkBytes)
	if err != nil {
		cv.csReactor.logger.Error("Decode Block failed")
		return false
	}
	h := blk.Header()
	cv.csReactor.logger.Debug("Decoded Block",
		"id", h.ID(),
		"parentID", h.ParentID(),
		"height", h.LastKBlockHeight(),
		"timestamp", h.Timestamp())

	// receive valid proposal. fully in this round.
	cv.nextRoundExpectationCancel()
	cv.nextRoundExpectationStart(cv.csReactor.curRound+1, NEW_ROUND_EXPECT_TIMEOUT)

	isKBlock := (ch.MsgSubType == PROPOSE_MSG_SUBTYPE_KBLOCK)
	// TBD: Validate block
	if isKBlock {
		if blk.Header().BlockType() != block.BLOCK_TYPE_K_BLOCK {
			cv.csReactor.logger.Error("block type check failed ...")
			return false
		}

		// TODO: wait for API
		//if blockchain.ValidateKBlock(kblock) == false {
		//      cv.csReactor.logger.Error("validate Kblock failed")
		//      return false
		//}
	} else {
		if blk.Header().BlockType() != block.BLOCK_TYPE_M_BLOCK {
			cv.csReactor.logger.Error("block type check failed ...")
			return false
		}

		//ValidateMBlock(block, size)
	}

	// update cspeers, build consensus peer topology
	// Right now is HUB topology, simply point back to proposer or leader
	// cv.AddcsPeer(src.netAddr)

	// Block is OK, send back voting
	proposerPubKey, err := crypto.UnmarshalPubkey(proposalMsg.ProposerID)
	if err != nil {
		cv.csReactor.logger.Error("ummarshal proposer public key of sender failed ")
		return false
	}

	cv.RemoveAllcsPeers()

	peers, err := cv.csReactor.GetMyPeers()
	for _, p := range peers {
		cv.csPeers = append(cv.csPeers, p)
	}

	offset := proposalMsg.SignOffset
	length := proposalMsg.SignLength
	signMsg := cv.csReactor.BuildProposalBlockSignMsg(*proposerPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))
	// fmt.Println("offset & legnth: ", offset, length, " sign Msg:", signMsg)

	sign := cv.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := cv.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))
	msg := cv.GenerateVoteForProposalMessage(sign, msgHash)

	// fmt.Println("VoterForProposal Message: ", msg.String())
	var m ConsensusMessage = msg

	proposerNetAddr := cv.csReactor.getCurrentProposer().NetAddr
	cv.SendMsgToPeer(&m, proposerNetAddr)

	cv.state = COMMITTEE_VALIDATOR_COMMITSENT

	var srcMsg ConsensusMessage = *proposalMsg
	// forward the message to peers
	cv.SendMsg(&srcMsg)

	return true
}

// process notary Block Messag
func (cv *ConsensusValidator) ProcessNotaryBlockMessage(notaryMsg *NotaryBlockMessage, src *ConsensusPeer) bool {
	// only process this message at the state of commitsent
	if cv.state != COMMITTEE_VALIDATOR_COMMITSENT {
		cv.csReactor.logger.Error("only process Notary in state", "expected", "COMMITTEE_VALIDATOR_COMMITSENT", "actual",
			cv.state)
		return false
	}

	// valid the common header first
	/****
	notaryMsg, ok := interface{}(notary).(NotaryBlockMessage)
	if ok != false {
		cv.csReactor.logger.Error("Message type is not NotaryBlockMessage")
		return false
	}
	****/

	ch := notaryMsg.CSMsgCommonHeader
	if !cv.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_NOTARY_BLOCK {
		cv.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_NOTARY_BLOCK")
		return false
	}

	// valid the senderindex is leader from the publicKey
	if bytes.Equal(ch.Sender, notaryMsg.ProposerID) == false {
		cv.csReactor.logger.Error("Proposal sender and proposalID mismatch")
		return false
	}

	if cv.csReactor.ValidateCMheaderSig(&ch, notaryMsg.SigningHash().Bytes()) == false {
		cv.csReactor.logger.Error("Signature validate failed")
		return false
	}

	// XXX: Now validate voter bitarray and aggaregated signature.

	// Block is OK, send notary voting back
	notaryPubKey, err := crypto.UnmarshalPubkey(notaryMsg.ProposerID)
	if err != nil {
		cv.csReactor.logger.Error("ummarshal proposer public key of sender failed ")
		return false
	}

	offset := notaryMsg.SignOffset
	length := notaryMsg.SignLength

	signMsg := cv.csReactor.BuildNotaryBlockSignMsg(*notaryPubKey, uint32(ch.MsgSubType), uint64(ch.Height), uint32(ch.Round))

	sign := cv.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := cv.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))
	msg := cv.GenerateVoteForNotaryMessage(sign, msgHash, VOTE_FOR_NOTARY_BLOCK)

	var m ConsensusMessage = msg
	proposerNetAddr := cv.csReactor.getCurrentProposer().NetAddr
	cv.SendMsgToPeer(&m, proposerNetAddr)

	var srcMsg ConsensusMessage = *notaryMsg
	// forward messages to peers
	cv.SendMsg(&srcMsg)

	// send back the notary and expect next round
	cv.nextRoundExpectationCancel()
	cv.nextRoundExpectationStart(cv.csReactor.curRound+1, NEW_ROUND_EXPECT_TIMEOUT)

	return true
}

// process notary Block Message
func (cv *ConsensusValidator) ProcessNotaryAnnounceMessage(notaryMsg *NotaryAnnounceMessage, src *ConsensusPeer) bool {
	// only process this message at the state of commitsent
	if cv.state != COMMITTEE_VALIDATOR_COMMITSENT {
		cv.csReactor.logger.Error("only process Notary in state", "expected", "COMMITTEE_VALIDATOR_COMMITSENT", "actual",
			cv.state)
		return false
	}

	// valid the common header first
	/***
	notaryMsg, ok := interface{}(notary).(NotaryAnnounceMessage)
	if ok != false {
		cv.csReactor.logger.Error("Message type is not NotaryBlockMessage")
		return false
	}
	***/

	ch := notaryMsg.CSMsgCommonHeader
	if !cv.checkHeightAndRound(ch) {
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_NOTARY_ANNOUNCE {
		cv.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_NOTARY_ANNOUNCE")
		return false
	}

	// valid the senderindex is leader from the publicKey
	if bytes.Equal(ch.Sender, notaryMsg.AnnouncerID) == false {
		cv.csReactor.logger.Error("Proposal sender and proposalID mismatch")
		return false
	}

	if cv.csReactor.ValidateCMheaderSig(&ch, notaryMsg.SigningHash().Bytes()) == false {
		cv.csReactor.logger.Error("Signature validate failed")
		return false
	}

	// Now the notary Announce message is OK

	// Validate Actual Committee
	if notaryMsg.CommitteeActualSize != len(notaryMsg.CommitteeActualMembers) {
		cv.csReactor.logger.Error("ActualCommittee length mismatch ...")
		return false
	}

	// Update the curActualCommittee by receving Notary message
	cv.csReactor.curActualCommittee = cv.csReactor.BuildCommitteeMemberFromInfo(cv.csReactor.csCommon.system, notaryMsg.CommitteeActualMembers)

	// TBD: validate announce bitarray & signature

	// Block is OK, send back voting
	announcerPubKey, err := crypto.UnmarshalPubkey(notaryMsg.AnnouncerID)
	if err != nil {
		cv.csReactor.logger.Error("ummarshal announcer public key of sender failed ")
		return false
	}
	offset := notaryMsg.SignOffset
	length := notaryMsg.SignLength

	signMsg := cv.csReactor.BuildNotaryAnnounceSignMsg(*announcerPubKey, uint32(notaryMsg.CommitteeID), uint64(ch.Height), uint32(ch.Round))

	sign := cv.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := cv.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))
	msg := cv.GenerateVoteForNotaryMessage(sign, msgHash, VOTE_FOR_NOTARY_ANNOUNCE)

	var m ConsensusMessage = msg
	leaderNetAddr := src.netAddr
	cv.SendMsgToPeer(&m, leaderNetAddr)

	// XXX: Start pacemaker here at this time.
	// TODO: we should design a meessage like committee decided and ask every committee member start pacemaker
	cv.csReactor.csPacemaker.Start(cv.csReactor.curHeight, 0)
	return true
}

// Generate VoteForProposal Message
func (cv *ConsensusValidator) GenerateVoteForNotaryMessage(sig bls.Signature, msgHash [32]byte, MsgSubType byte) *VoteForNotaryMessage {

	curHeight := cv.csReactor.curHeight
	curRound := cv.csReactor.curRound

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      curRound,
		Sender:     crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_VOTE_FOR_NOTARY,
		MsgSubType: MsgSubType,
	}

	index := cv.csReactor.GetCommitteeMemberIndex(cv.csReactor.myPubKey)
	msg := &VoteForNotaryMessage{
		CSMsgCommonHeader: cmnHdr,

		VoterID:           crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		CSVoterPubKey:     cv.csReactor.csCommon.system.PubKeyToBytes(cv.csReactor.csCommon.PubKey),
		VoterSignature:    cv.csReactor.csCommon.system.SigToBytes(sig), //TBD
		VoterIndex:        index,
		SignedMessageHash: msgHash,
	}

	// sign message
	msgSig, err := cv.csReactor.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		cv.csReactor.logger.Error("Sign message failed", "error", err)
		return nil
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)
	cv.csReactor.logger.Debug("Generate Voter For Notary Message", "msg", msg.String())
	return msg
}

// Process the MoveNewRoundMessage
func (cv *ConsensusValidator) ProcessMoveNewRoundMessage(newRoundMsg *MoveNewRoundMessage, src *ConsensusPeer) bool {
	// only process this message at the state of commitsent
	if cv.state != COMMITTEE_VALIDATOR_COMMITSENT {
		cv.csReactor.logger.Error("only process Notary in state", "expected", "COMMITTEE_VALIDATOR_COMMITSENT", "actual",
			cv.state)
		return false
	}

	ch := newRoundMsg.CSMsgCommonHeader

	if cv.csReactor.ValidateCMheaderSig(&ch, newRoundMsg.SigningHash().Bytes()) == false {
		cv.csReactor.logger.Error("Signature validate failed")
		return false
	}

	chainCurHeight := int64(cv.csReactor.chain.BestBlock().Header().Number())
	cv.csReactor.logger.Debug("ProcessMoveNewRound", "Chain curHeight", chainCurHeight, "Reactor curHeight", cv.csReactor.curHeight)
	if ch.Height != cv.csReactor.curHeight {
		cv.csReactor.logger.Error("Height mismatch!", "curHeight", cv.csReactor.curHeight, "incomingHeight", ch.Height)
		return false
	}

	if ch.Round != 0 && ch.Round != cv.csReactor.curRound {
		cv.csReactor.logger.Error("Round mismatch!", "curRound", cv.csReactor.curRound, "incomingRound", ch.Round)

		// since the height is the same, it is possible we missed some rounds, pace the round!
		if ch.Round > cv.csReactor.curRound {
			cv.csReactor.curRound = ch.Round
		} else {
			return false
		}
	}

	if ch.MsgType != CONSENSUS_MSG_MOVE_NEW_ROUND {
		cv.csReactor.logger.Error("MsgType is not CONSENSUS_MSG_MOVE_NEW_ROUND")
		return false
	}

	// TBD: sender must be current proposer or leader
	/****
	if ch.Sender != newRoundMsg.CurProposer {
		cv.csReactor.logger.Error("Proposal sender and proposalID mismatch")
		return false
	}
	***/

	if cv.csReactor.curRound != newRoundMsg.CurRound {
		cv.csReactor.logger.Error("curRound mismatch!", "curRound", cv.csReactor.curRound, "newRoundMsg.CurRound", newRoundMsg.CurRound)
		return false
	}

	if cv.csReactor.curHeight != newRoundMsg.Height {
		cv.csReactor.logger.Error("curHeight mismatch!", "curHeight", cv.csReactor.curHeight, "newRoundMsg.CurHeight", newRoundMsg.Height)
		return false
	}

	//HACK for test: Right now the ActualCommittee is empty.

	// Now validate current proposer and new proposer
	curProposer := cv.csReactor.getCurrentProposer()
	if bytes.Equal(crypto.FromECDSAPub(&curProposer.PubKey), newRoundMsg.CurProposer) == false {
		cv.csReactor.logger.Error("curProposer mismacth!")
		return false
	}

	newProposer := cv.csReactor.getRoundProposer(newRoundMsg.NewRound)
	if bytes.Equal(crypto.FromECDSAPub(&newProposer.PubKey), newRoundMsg.NewProposer) == false {
		cv.csReactor.logger.Error("newProposer mismacth!")
		return false
	}

	cv.csReactor.UpdateHeightRound(newRoundMsg.Height, newRoundMsg.NewRound)

	// relay message to peers first
	var m ConsensusMessage = newRoundMsg
	cv.SendMsg(&m)

	// Have move to next round, check I am the new round proposer
	if bytes.Equal(crypto.FromECDSAPub(&newProposer.PubKey), crypto.FromECDSAPub(&cv.csReactor.myPubKey)) {
		cv.csReactor.logger.Info("*** I AM THE PROPOSER! ***", "height", newRoundMsg.Height, "round", newRoundMsg.NewRound)

		//start next round expectation
		cv.nextRoundExpectationCancel()
		cv.csReactor.ScheduleProposer(PROPOSER_THRESHOLD_TIMER_TIMEOUT)
		return true
	}

	//start next round expectation
	cv.nextRoundExpectationCancel()
	cv.nextRoundExpectationStart(cv.csReactor.curRound, NEW_ROUND_EXPECT_TIMEOUT)
	return true
}

func (cv *ConsensusValidator) checkHeightAndRound(ch ConsensusMsgCommonHeader) bool {
	if ch.Height != cv.csReactor.curHeight {
		cv.csReactor.logger.Error("Height mismatch!", "curHeight", cv.csReactor.curHeight, "incomingHeight", ch.Height)
		return false
	}

	if ch.Round != 0 && ch.Round != cv.csReactor.curRound {
		cv.csReactor.logger.Error("Round mismatch!", "curRound", cv.csReactor.curRound, "incomingRound", ch.Round)
		return false
	}
	return true
}
