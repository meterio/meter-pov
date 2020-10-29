// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

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
	"encoding/base64"
	"fmt"
	"time"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/genesis"
	types "github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

// for all committee mermbers
const (
	// FSM of VALIDATOR
	COMMITTEE_VALIDATOR_INIT       = byte(0x01)
	COMMITTEE_VALIDATOR_COMMITSENT = byte(0x02)
)

type ConsensusValidator struct {
	replay  bool
	EpochID uint64 // epoch ID of this committee

	csReactor    *ConsensusReactor //global reactor info
	state        byte
	csPeers      []*ConsensusPeer // consensus message peers
	newCommittee NewCommittee
}

func (cv *ConsensusValidator) SendMsgToPeer(msg *ConsensusMessage, netAddr types.NetAddress) bool {
	name := cv.csReactor.GetDelegateNameByIP(netAddr.IP)
	csPeer := newConsensusPeer(name, netAddr.IP, netAddr.Port, cv.csReactor.magic)
	return cv.csReactor.asyncSendCommitteeMsg(msg, false, csPeer)
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

// Generate commitCommittee Message
func (cv *ConsensusValidator) GenerateCommitMessage(sig bls.Signature, msgHash [32]byte, round uint32) *CommitCommitteeMessage {

	curHeight := cv.csReactor.curHeight

	cmnHdr := ConsensusMsgCommonHeader{
		Height:    curHeight,
		Round:     round,
		Sender:    crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		Timestamp: time.Now(),
		MsgType:   CONSENSUS_MSG_COMMIT_COMMITTEE,
		EpochID:   cv.EpochID,
	}

	msg := &CommitCommitteeMessage{
		CSMsgCommonHeader: cmnHdr,
		CommitterID:       crypto.FromECDSAPub(&cv.csReactor.myPubKey),
		CommitterBlsPK:    cv.csReactor.csCommon.GetSystem().PubKeyToBytes(*cv.csReactor.csCommon.GetPublicKey()), //bls pubkey
		BlsSignature:      cv.csReactor.csCommon.GetSystem().SigToBytes(sig),
		CommitterIndex:    cv.csReactor.curCommitteeIndex,
		SignedMsgHash:     msgHash,
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
func (cv *ConsensusValidator) ProcessAnnounceCommittee(announceMsg *AnnounceCommitteeMessage, src *ConsensusPeer) bool {

	// only process this message at the state of init
	if cv.state != COMMITTEE_VALIDATOR_INIT {
		cv.csReactor.logger.Error("only process announcement in state", "expected", "COMMITTEE_VALIDATOR_INIT", "actual",
			cv.state)
		return true
	}

	ch := announceMsg.CSMsgCommonHeader
	if ch.Height != cv.csReactor.curHeight {
		cv.csReactor.logger.Error("Height mismatch!", "curHeight", cv.csReactor.curHeight, "incomingHeight", ch.Height)
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
	round := cv.csReactor.newCommittee.Round % uint32(len(cv.csReactor.newCommittee.Committee.Validators))
	lv := cv.csReactor.curCommittee.Validators[round]
	if bytes.Equal(crypto.FromECDSAPub(&lv.PubKey), announceMsg.AnnouncerID) == false {
		cv.csReactor.logger.Warn("Sender is not leader ...",
			"sender", base64.StdEncoding.EncodeToString(announceMsg.AnnouncerID), "round", ch.Round,
			"expect leader", base64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&lv.PubKey)), "expect round", round)

		// we know that they reach majority in a round other than mine. if I am still out, join committee.
		// 1. I did not join other committee
		// 2. no pacemaker running
		if cv.state != COMMITTEE_VALIDATOR_COMMITSENT && cv.csReactor.IsPacemakerRunning() == false {
			cv.csReactor.NewCommitteeUpdateRound(ch.Round)
			cv.csReactor.logger.Info("adjusted my round, continue to process Announce", "from", round, "to", ch.Round)
		} else {
			return false
		}
	}
	if bytes.Equal(cv.csReactor.csCommon.GetSystem().PubKeyToBytes(lv.BlsPubKey), announceMsg.AnnouncerBlsPK) == false {
		cv.csReactor.logger.Error("bls public key mismatch", "round", round)
		return false
	}
	cv.csReactor.logger.Info("Committee announced by", "peer", lv.Name, "ip", lv.NetAddr.IP.String())

	// update cspeers, build consensus peer topology
	// Right now is HUB topology, simply point back to proposer or leader
	cv.RemoveAllcsPeers()

	cv.EpochID = announceMsg.EpochID()

	// I am in committee, sends the commit message to join the CommitCommitteeMessage
	signMsg := cv.csReactor.BuildAnnounceSignMsg(lv.PubKey, announceMsg.EpochID(), uint64(ch.Height), uint32(ch.Round))
	sign, msgHash := cv.csReactor.csCommon.SignMessage([]byte(signMsg))
	msg := cv.GenerateCommitMessage(sign, msgHash, cv.csReactor.newCommittee.Round)

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

	ch := notaryMsg.CSMsgCommonHeader
	if ch.Height != cv.csReactor.curHeight {
		cv.csReactor.logger.Error("Height mismatch!", "curHeight", cv.csReactor.curHeight, "incomingHeight", ch.Height)
		return false
	}

	// valid the senderindex is leader from the publicKey
	if bytes.Equal(ch.Sender, notaryMsg.AnnouncerID) == false {
		cv.csReactor.logger.Error("Proposal sender and proposalID mismatch")
		return false
	}

	// valid the voter index. we can get the index from the publicKey
	senderPubKey, err := crypto.UnmarshalPubkey(ch.Sender)
	if err != nil {
		cv.csReactor.logger.Error("ummarshal public key of sender failed ")
		return false
	}

	leaderIndex := cv.csReactor.GetCommitteeMemberIndex(*senderPubKey)
	if leaderIndex < 0 {
		cv.csReactor.logger.Error("get leader index failed.")
		return false
	}
	leader := cv.csReactor.curCommittee.Validators[uint32(leaderIndex)]
	if bytes.Equal(crypto.FromECDSAPub(&leader.PubKey), notaryMsg.AnnouncerID) == false {
		cv.csReactor.logger.Error("ecdsa public key mismatch", "index", leaderIndex)
		return false
	}
	if bytes.Equal(cv.csReactor.csCommon.GetSystem().PubKeyToBytes(leader.BlsPubKey), notaryMsg.AnnouncerBlsPK) == false {
		cv.csReactor.logger.Error("bls public key mismatch", "index", leaderIndex)
		return false
	}

	// TBD: validate announce bitarray & signature
	// validateEvidence()

	cv.csReactor.UpdateActualCommittee(uint32(leaderIndex), cv.csReactor.config)
	myCommitteInfo := cv.csReactor.BuildCommitteeInfoFromMember(cv.csReactor.csCommon.GetSystem(), cv.csReactor.curActualCommittee)

	// my committee info notaryMsg.CommitteeMembers must be the same !!!
	if cv.csReactor.CommitteeInfoCompare(myCommitteInfo, notaryMsg.CommitteeMembers) == false {
		cv.csReactor.logger.Error("CommitteeInfo mismatch")
		return false
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
	err = cv.csReactor.startPacemaker(newCommittee, PMModeNormal)
	if err != nil {
		fmt.Println("could not start pacemaker, error:", err)
		return false
	}
	return true
}
