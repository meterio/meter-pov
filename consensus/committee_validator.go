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
	"bytes"
	//	"crypto/ecdsa"
	"time"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	crypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dfinlab/meter/genesis"
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
	replay  bool
	EpochID uint64 // epoch ID of this committee

	csReactor    *ConsensusReactor //global reactor info
	state        byte
	csPeers      []*ConsensusPeer // consensus message peers
	newCommittee NewCommittee
}

// send consensus message to all connected peers
func (cv *ConsensusValidator) SendMsg(msg *ConsensusMessage) bool {
	return cv.csReactor.SendMsgToPeers(cv.csPeers, msg)
}

func (cv *ConsensusValidator) SendMsgToPeer(msg *ConsensusMessage, netAddr types.NetAddress) bool {
	name := cv.csReactor.GetCommitteeMemberNameByIP(netAddr.IP)
	csPeer := newConsensusPeer(name, netAddr.IP, netAddr.Port, cv.csReactor.magic)
	return cv.csReactor.SendMsgToPeers([]*ConsensusPeer{csPeer}, msg)
}

//validator receives the initiated messages
func NewConsensusValidator(conR *ConsensusReactor) *ConsensusValidator {
	var cv ConsensusValidator

	// initialize the ConsenusLeader
	cv.state = COMMITTEE_VALIDATOR_INIT
	cv.csReactor = conR

	return &cv
}

// validator need to build consensus peer topology
func (cv *ConsensusValidator) RemoveAllcsPeers() bool {
	cv.csPeers = []*ConsensusPeer{}
	return true
}

/* deprecated
func (cv *ConsensusValidator) AddcsPeer(netAddr types.NetAddress) bool {
	csPeer := newConsensusPeer(netAddr.IP, netAddr.Port, cv.csReactor.magic)
	cv.csPeers = append(cv.csPeers, csPeer)
	return true
}
*/

// Generate commitCommittee Message
func (cv *ConsensusValidator) GenerateCommitMessage(sig bls.Signature, msgHash [32]byte) *CommitCommitteeMessage {

	curHeight := cv.csReactor.curHeight

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     0,
		Sender:    crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_COMMIT_COMMITTEE,
		EpochID:   cv.EpochID,
	}

	index := cv.csReactor.GetCommitteeMemberIndex(cv.csReactor.myPubKey)
	msg := &CommitCommitteeMessage{
		CSMsgCommonHeader: cmnHdr,

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

	ch := announceMsg.CSMsgCommonHeader
	if !cv.csReactor.checkHeight(ch) {
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

	//get the nonce
	kblock, err := cv.csReactor.chain.GetTrunkBlock(uint32(announceMsg.KBlockHeight))
	var kblockNonce uint64
	if err != nil {
		cv.csReactor.logger.Warn("Could not get KBlock, use nonce from announce message", "nonce", announceMsg.Nonce)
		kblockNonce = announceMsg.Nonce
	} else {
		if kblock.Header().Number() == 0 {
			kblockNonce = genesis.GenesisNonce
		} else {
			kblockNonce = kblock.KBlockData.Nonce
		}
		if announceMsg.Nonce != kblockNonce {
			cv.csReactor.logger.Error("Nonce mismatch, potential malicious behaviour...", "kblockNonce", kblockNonce, "recvedNonce", announceMsg.Nonce)
			return false
		}
	}
	// Now the announce message is OK

	role, inCommittee := cv.csReactor.NewValidatorSetByNonce(kblockNonce)
	if !inCommittee {
		cv.csReactor.logger.Error("I am not in committee, do nothing ...")
		return false
	}

	if (role != CONSENSUS_COMMIT_ROLE_VALIDATOR) && (role != CONSENSUS_COMMIT_ROLE_LEADER) {
		cv.csReactor.logger.Error("I am not the validtor/leader of committee ...")
		return false
	}

	// Verify Leader is announce sender?  should match my round
	round := cv.csReactor.newCommittee.Round % uint64(len(cv.csReactor.newCommittee.Committee.Validators))
	lv := cv.csReactor.curCommittee.Validators[round]
	if bytes.Equal(crypto.FromECDSAPub(&lv.PubKey), ch.Sender) == false {
		cv.csReactor.logger.Error("Sender is not leader in my committee ...")
		return false
	}
	cv.csReactor.logger.Info("Committee announced by", "peer", lv.Name, "ip", lv.NetAddr.IP.String())

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
		//cv.replay = false
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
	signMsg := cv.csReactor.BuildAnnounceSignMsg(*announcerPubKey, announceMsg.CSMsgCommonHeader.EpochID, uint64(ch.Height), uint32(ch.Round))
	// fmt.Println("offset & length: ", offset, length, "sign msg:", signMsg)

	if int(offset+length) > len(signMsg) {
		cv.csReactor.logger.Error("out of boundary ...")
		return false
	}

	cv.EpochID = announceMsg.CSMsgCommonHeader.EpochID

	sign := cv.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := cv.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))
	msg := cv.GenerateCommitMessage(sign, msgHash)

	var m ConsensusMessage = msg
	cv.SendMsgToPeer(&m, lv.NetAddr)
	cv.state = COMMITTEE_VALIDATOR_COMMITSENT

	//update conR
	cv.csReactor.updateCurEpoch(cv.EpochID)
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
	if !cv.csReactor.checkHeight(ch) {
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

	found := false
	for _, c := range cv.csReactor.curActualCommittee {
		if bytes.Equal(crypto.FromECDSAPub(&c.PubKey), crypto.FromECDSAPub(&cv.csReactor.myPubKey)) == true {
			found = true
			break
		}
	}
	if !found {
		cv.csReactor.logger.Error("I'm not in ActualCommittee, ignore this notary msg ...")
		return false
	}

	var lv *types.Validator
	for _, v := range cv.csReactor.curCommittee.Validators {
		keyBytes := crypto.FromECDSAPub(&v.PubKey)
		if bytes.Compare(keyBytes, notaryMsg.AnnouncerID) == 0 {
			lv = v
			break
		}
	}
	if lv == nil {
		cv.csReactor.logger.Error("announcer is not in my committee")
		return false
	}
	cv.csReactor.logger.Info("Notary announced by", "peer", lv.Name, "ip", lv.NetAddr.IP.String())

	// TBD: validate announce bitarray & signature

	// Block is OK, send back voting
	announcerPubKey, err := crypto.UnmarshalPubkey(notaryMsg.AnnouncerID)
	if err != nil {
		cv.csReactor.logger.Error("ummarshal announcer public key of sender failed ")
		return false
	}
	offset := notaryMsg.SignOffset
	length := notaryMsg.SignLength

	signMsg := cv.csReactor.BuildNotaryAnnounceSignMsg(*announcerPubKey, notaryMsg.CSMsgCommonHeader.EpochID, uint64(ch.Height), uint32(ch.Round))

	sign := cv.csReactor.csCommon.SignMessage([]byte(signMsg), uint32(offset), uint32(length))
	msgHash := cv.csReactor.csCommon.Hash256Msg([]byte(signMsg), uint32(offset), uint32(length))
	msg := cv.GenerateVoteForNotaryMessage(sign, msgHash, VOTE_FOR_NOTARY_ANNOUNCE)

	// lv := cv.csReactor.curCommittee.Validators[round]

	if lv != nil {
		var m ConsensusMessage = msg
		cv.SendMsgToPeer(&m, lv.NetAddr)
	}

	// stop new committee timer cos it is established
	cv.csReactor.NewCommitteeCleanup()

	//Committee is established. Myself is Leader, server as 1st proposer.
	cv.csReactor.logger.Info(`
===========================================================
Committee is established!!! ...
Let's start the pacemaker...
===========================================================`, "Committee Epoch", cv.csReactor.curEpoch)

	// XXX: Start pacemaker here at this time.
	newCommittee := !cv.replay
	//cv.csReactor.csPacemaker.Start(newCommittee)
	cv.csReactor.startPacemaker(newCommittee)
	return true
}

// Generate VoteForProposal Message
func (cv *ConsensusValidator) GenerateVoteForNotaryMessage(sig bls.Signature, msgHash [32]byte, MsgSubType byte) *VoteForNotaryMessage {

	curHeight := cv.csReactor.curHeight

	cmnHdr := ConsensusMsgCommonHeader{
		Height:     curHeight,
		Round:      0,
		Sender:     crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		Timestamp:  time.Now(),
		MsgType:    CONSENSUS_MSG_VOTE_FOR_NOTARY,
		MsgSubType: MsgSubType,

		EpochID: cv.EpochID,
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
