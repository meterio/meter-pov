// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"

	//sha256 "crypto/sha256"
	"time"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	types "github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	NEW_COMMITTEE_INIT_INTV = 15 * time.Second //15s
)

type NCEvidence struct {
	voterBitArray *cmn.BitArray
	voterMsgHash  [32]byte
	voterAggSig   bls.Signature
}

func NewNCEvidence(bitArray *cmn.BitArray, msgHash [32]byte, aggSig bls.Signature) *NCEvidence {
	return &NCEvidence{
		voterBitArray: bitArray,
		voterMsgHash:  msgHash,
		voterAggSig:   aggSig,
	}
}

type NewCommitteeKey struct {
	height uint32
	round  uint32
	epoch  uint64
}

type NewCommittee struct {
	KblockHeight uint32
	Round        uint32
	Nonce        uint64
	Replay       bool
	TimeoutTimer *time.Timer

	// // evidence
	// voterBitArray *cmn.BitArray
	// voterPubKey   []bls.PublicKey
	// voterAggSig   bls.Signature

	// // intermediate results
	// voterSig     []bls.Signature
	// voterMsgHash [][32]byte
	// voterNum     int

	sigAggregator *SignatureAggregator

	//precalc committee for given nonce
	Committee   *types.ValidatorSet
	Role        uint //leader, validator, none
	Index       int  //the index of delegates
	InCommittee bool
}

func newNewCommittee(height, round uint32, nonce uint64) *NewCommittee {
	return &NewCommittee{
		KblockHeight: height,
		Nonce:        nonce,
		Round:        round,
		TimeoutTimer: nil,
	}
}

func (conR *ConsensusReactor) NewCommitteeTimeout() {
	// increase round
	conR.newCommittee.Round++
	if conR.newCommittee.InCommittee {
		// wait for long time, check if it is possible committee already established
		if conR.newCommittee.Round >= 3 {
			if conR.CheckEstablishedCommittee(conR.newCommittee.KblockHeight) == true {
				conR.NewCommitteeTimerStop()
				conR.logger.Info("found out committee established, join the committee.",
					"round", conR.newCommittee.Round, "kblockHeight", conR.newCommittee.KblockHeight)

				kblock, err := conR.chain.GetTrunkBlock(conR.newCommittee.KblockHeight)
				if err != nil {
					conR.logger.Error("get last kblock block error", "height", conR.newCommittee.KblockHeight, "error", err)
					return
				}

				conR.JoinEstablishedCommittee(kblock, false)
				return
			}
		}

		size := len(conR.newCommittee.Committee.Validators)
		nl := conR.newCommittee.Committee.Validators[conR.newCommittee.Round%uint32(size)]

		leader := newConsensusPeer(nl.Name, nl.NetAddr.IP, nl.NetAddr.Port, conR.magic)
		leaderPubKey := nl.PubKey
		conR.sendNewCommitteeMessage(leader, leaderPubKey, conR.newCommittee.KblockHeight,
			conR.newCommittee.Nonce, conR.newCommittee.Round)

		conR.NewCommitteeTimerStart()
		conR.logger.Warn("Committee Timeout, sent newcommittee msg", "peer", leader.name, "ip", leader.String(), "round", conR.newCommittee.Round)
		if conR.csValidator != nil {
			conR.csValidator.state = COMMITTEE_VALIDATOR_INIT
		}
	} else {
		conR.NewCommitteeTimerStop()
		conR.logger.Warn("Committee Timeout, not in newcommtteesent newcommittee:")
	}
}

func (conR *ConsensusReactor) NewCommitteeInit(height uint32, nonce uint64, replay bool) {
	conR.logger.Info("NewCommitteeInit ...", "height", height, "nonce", nonce, "replay", replay)
	nc := newNewCommittee(height, 0, nonce)
	nc.Replay = replay
	//pre-calc the committee
	committee, role, index, inCommittee := conR.CalcCommitteeByNonce(nonce)
	nc.Committee = committee
	nc.Role = role
	nc.Index = index
	nc.InCommittee = inCommittee

	//assign
	conR.NewCommitteeTimerStop() // stop the previous timer before replace with current one
	conR.newCommittee = nc
}

func (conR *ConsensusReactor) NewCommitteeCleanup() {
	// clean up received map
	for k, _ := range conR.rcvdNewCommittee {
		delete(conR.rcvdNewCommittee, k)
	}

	// clean up local
	conR.NewCommitteeTimerStop()
	conR.newCommittee.Replay = false
	conR.newCommittee.InCommittee = false
	conR.newCommittee.Role = CONSENSUS_COMMIT_ROLE_NONE
}

func (conR *ConsensusReactor) NewCommitteeTimerStart() {
	conR.NewCommitteeTimerStop()
	timeoutInterval := NEW_COMMITTEE_INIT_INTV * (2 << conR.newCommittee.Round)
	conR.newCommittee.TimeoutTimer = time.AfterFunc(timeoutInterval, func() {
		conR.schedulerQueue <- func() { conR.NewCommitteeTimeout() }
	})
}

func (conR *ConsensusReactor) NewCommitteeTimerStop() {
	if conR.newCommittee == nil {
		return
	}
	if conR.newCommittee.TimeoutTimer != nil {
		conR.newCommittee.TimeoutTimer.Stop()
		conR.newCommittee.TimeoutTimer = nil
	}
}

func (conR *ConsensusReactor) NewCommitteeUpdateRound(round uint32) {
	if conR.newCommittee == nil {
		return
	}
	conR.NewCommitteeTimerStop()
	conR.newCommittee.Round = round
	timeoutInterval := NEW_COMMITTEE_INIT_INTV * (2 << conR.newCommittee.Round)
	conR.newCommittee.TimeoutTimer = time.AfterFunc(timeoutInterval, func() {
		conR.schedulerQueue <- func() { conR.NewCommitteeTimeout() }
	})
}

func (conR *ConsensusReactor) updateCurEpoch(epoch uint64) {
	if epoch > conR.curEpoch {
		oldVal := conR.curEpoch
		conR.curEpoch = epoch
		curEpochGauge.Set(float64(conR.curEpoch))
		conR.logger.Info("Epoch updated", "to", conR.curEpoch, "from", oldVal)
	}
}

// NewcommitteeMessage routines
// send new round message to future committee leader
func (conR *ConsensusReactor) sendNewCommitteeMessage(peer *ConsensusPeer, leaderPubKey ecdsa.PublicKey, kblockHeight uint32, nonce uint64, round uint32) bool {
	conR.updateCurEpoch(conR.chain.BestBlock().QC.EpochID)

	// keep the epoch is the same if it is the replay
	var nextEpochID uint64
	nextEpochID = conR.curEpoch + 1
	if conR.newCommittee.Replay == true {
		nextEpochID = conR.curEpoch
	}

	msg := &NewCommitteeMessage{
		CSMsgCommonHeader: ConsensusMsgCommonHeader{
			Height:    conR.curHeight,
			Round:     round,
			Sender:    crypto.FromECDSAPub(&conR.myPubKey),
			Timestamp: time.Now(),
			MsgType:   CONSENSUS_MSG_NEW_COMMITTEE,
			EpochID:   conR.curEpoch,
		},

		NextEpochID:    nextEpochID,
		NewLeaderID:    crypto.FromECDSAPub(&leaderPubKey),
		ValidatorID:    crypto.FromECDSAPub(&conR.myPubKey),
		ValidatorBlsPK: conR.csCommon.GetSystem().PubKeyToBytes(*conR.csCommon.GetPublicKey()),
		Nonce:          nonce,
		KBlockHeight:   kblockHeight,
	}

	// sign message with bls key
	signMsg := conR.BuildNewCommitteeSignMsg(leaderPubKey, nextEpochID, uint64(conR.curHeight))
	blsSig, msgHash := conR.csCommon.SignMessage2([]byte(signMsg))
	msg.BlsSignature = blsSig
	msg.SignedMsgHash = msgHash

	// sign message with ecdsa key
	ecdsaSigBytes, err := conR.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		conR.logger.Error("Sign message failed", "error", err)
		return false
	}
	msg.CSMsgCommonHeader.SetMsgSignature(ecdsaSigBytes)

	// state to init & send move to next round
	// fmt.Println("msg: %v", msg.String())
	var m ConsensusMessage = msg
	return conR.asyncSendCommitteeMsg(&m, false, peer)
}

// once reach 2/3 send aout annouce message
func (conR *ConsensusReactor) ProcessNewCommitteeMessage(newCommitteeMsg *NewCommitteeMessage, src *ConsensusPeer) bool {
	// conR.logger.Info("received newCommittee Message", "source", src.name, "IP", src.netAddr.IP)
	ch := newCommitteeMsg.CSMsgCommonHeader

	// non replay case, last block must be kblock
	if conR.newCommittee.Replay == false && ch.Height != newCommitteeMsg.KBlockHeight {
		conR.logger.Error("CurHeight is not the same with kblock height")
		return false
	}

	if bytes.Equal(ch.Sender, newCommitteeMsg.ValidatorID) == false {
		conR.logger.Error("sender / validatorID mismatch")
		return false
	}

	// check the leader
	if bytes.Compare(newCommitteeMsg.NewLeaderID, crypto.FromECDSAPub(&conR.myPubKey)) != 0 {
		conR.logger.Error("Expected leader is not myself")
		return false
	}

	// TODO: check & collect the vote
	validatorID, err := crypto.UnmarshalPubkey(newCommitteeMsg.ValidatorID)
	if err != nil {
		conR.logger.Error("validator key unmarshal error")
		return false
	}
	leaderID, err := crypto.UnmarshalPubkey(newCommitteeMsg.NewLeaderID)
	if err != nil {
		conR.logger.Error("leader key unmarshal error")
		return false
	}

	signMsg := conR.BuildNewCommitteeSignMsg(*leaderID, newCommitteeMsg.NextEpochID, uint64(ch.Height))
	conR.logger.Debug("Sign message", "msg", signMsg)

	// validate the message hash
	msgHash := conR.csCommon.Hash256Msg([]byte(signMsg))
	if msgHash != newCommitteeMsg.SignedMsgHash {
		conR.logger.Error("msgHash mismatch ...")
		return false
	}

	// validate the signature
	sig, err := conR.csCommon.GetSystem().SigFromBytes(newCommitteeMsg.BlsSignature)
	if err != nil {
		conR.logger.Error("get signature failed ...")
		return false
	}

	// sanity check done
	epochID := newCommitteeMsg.NextEpochID
	height := newCommitteeMsg.KBlockHeight
	round := ch.Round
	key := NewCommitteeKey{height, round, newCommitteeMsg.EpochID()}
	nonce := newCommitteeMsg.Nonce

	nc, ok := conR.rcvdNewCommittee[key]
	var newCommittee *types.ValidatorSet
	var inCommittee bool
	if ok == false {
		newCommittee, _, _, inCommittee = conR.CalcCommitteeByNonce(nonce)
		if !inCommittee {
			conR.logger.Info("I'm not in this committee, drop this newcommittee msg")
			return false
		}
	} else {
		newCommittee = nc.Committee
	}
	// check ECDSA pubkey
	var index int
	var validator *types.Validator
	for i := range newCommittee.Validators {
		v := newCommittee.Validators[i]
		pubkey := crypto.FromECDSAPub(&v.PubKey)
		if bytes.Equal(pubkey, newCommitteeMsg.ValidatorID) == true {
			validator = v
			index = i
			break
		}
	}
	pubkeyB64 := base64.StdEncoding.EncodeToString(crypto.FromECDSAPub(validatorID))
	if validator == nil {
		conR.logger.Error("invalid newcommittee msg, this validator is not in committee", "validatorPubKey", pubkeyB64, "nonce", nonce)
		return false
	}

	nc, ok = conR.rcvdNewCommittee[key]
	if ok == false {
		nc = newNewCommittee(height, round, nonce)
		committee, role, index, inCommittee := conR.CalcCommitteeByNonce(nonce)
		nc.Committee = committee
		nc.Role = role
		nc.Index = index
		nc.InCommittee = inCommittee
		nc.sigAggregator = newSignatureAggregator(uint32(len(nc.Committee.Validators)), *conR.csCommon.GetSystem(), msgHash, nc.Committee.Validators)

		conR.rcvdNewCommittee[key] = nc
	} else {
		// same height round NewCommittee is there
		if nc.Nonce != nonce {
			conR.logger.Error("nonce mismtach between message and reactor", "recevied", nonce, "have", nc.Nonce)
			return false
		}
	}

	if bytes.Equal(conR.csCommon.GetSystem().PubKeyToBytes(validator.BlsPubKey), newCommitteeMsg.ValidatorBlsPK) == false {
		blsPK := base64.StdEncoding.EncodeToString(conR.csCommon.GetSystem().PubKeyToBytes(validator.BlsPubKey))
		actualBlsPK := base64.StdEncoding.EncodeToString(newCommitteeMsg.ValidatorBlsPK)
		conR.logger.Error("BlsPubKey mismatch", "validator blsPK", blsPK, "actual blsPK", actualBlsPK)
		return false
	}

	// validate bls signature
	valid := bls.Verify(sig, msgHash, validator.BlsPubKey)
	if valid == false {
		conR.logger.Error("validate voter signature failed")
		if conR.config.SkipSignatureCheck == true {
			conR.logger.Error("but SkipSignatureCheck is true, continue ...")
		} else {
			return false
		}
	}

	// collect the votes
	nc.sigAggregator.Add(index, msgHash, newCommitteeMsg.BlsSignature, validator.BlsPubKey)
	// nc.voterNum++
	// nc.voterBitArray.SetIndex(index, true)
	// nc.voterSig = append(nc.voterSig, sig)
	// nc.voterPubKey = append(nc.voterPubKey, validator.BlsPubKey)
	// nc.voterMsgHash = append(nc.voterMsgHash, msgHash)

	// 3. if the totoal vote > 2/3, move to Commit state
	if LeaderMajorityTwoThird(nc.sigAggregator.Count(), conR.committeeSize) {
		conR.logger.Debug("NewCommitteeMessage, 2/3 Majority reached", "Recvd", nc.sigAggregator.Count(), "committeeSize", conR.committeeSize, "replay", conR.newCommittee.Replay)

		// seal the signature, avoid re-trigger
		nc.sigAggregator.Seal()

		// Now it's time schedule leader
		if conR.csPacemaker.IsStopped() == false {
			conR.csPacemaker.Stop()
		}

		// nc.voterNum = 0
		aggSigBytes := nc.sigAggregator.Aggregate()
		aggSig, _ := conR.csCommon.GetSystem().SigFromBytes(aggSigBytes)

		// aggregate the signatures
		// nc.voterAggSig = conR.csCommon.AggregateSign(nc.voterSig)

		if conR.newCommittee.Replay == true {
			conR.ScheduleReplayLeader(epochID, height, round,
				NewNCEvidence(nc.sigAggregator.bitArray, nc.sigAggregator.msgHash, aggSig),
				WHOLE_NETWORK_BLOCK_SYNC_TIME)
		} else {
			// Wait for block sync since there is no time out yet
			conR.ScheduleLeader(epochID, height, round,
				NewNCEvidence(nc.sigAggregator.bitArray, nc.sigAggregator.msgHash, aggSig),
				WHOLE_NETWORK_BLOCK_SYNC_TIME)
		}

		conR.logger.Info("Leader scheduled ...", "height", height, "round", round, "epoch", epochID)
		return true
	} else {
		// not reach 2/3 yet, wait for more
		conR.logger.Debug("received NewCommitteeMessage (2/3 not reached yet, wait for more)", "Recvd", nc.sigAggregator.Count(), "committeeSize", conR.committeeSize)
		return true
	}

}
