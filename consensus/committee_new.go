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
	height uint64
	round  uint64
}

type NewCommittee struct {
	KblockHeight uint64
	Round        uint64
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

func newNewCommittee(height, round, nonce uint64) *NewCommittee {
	return &NewCommittee{
		KblockHeight: height,
		Nonce:        nonce,
		Round:        round,
		TimeoutTimer: nil,
	}
}

func (conR *ConsensusReactor) NewCommitteeTimeout() error {
	// increase round
	conR.newCommittee.Round++
	if conR.newCommittee.InCommittee {
		size := len(conR.newCommittee.Committee.Validators)
		nl := conR.newCommittee.Committee.Validators[conR.newCommittee.Round%uint64(size)]

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
		conR.logger.Warn("Committee Timeout, not in newcommtteesent newcommittee:")
	}

	return nil
}

func (conR *ConsensusReactor) NewCommitteeInit(height, nonce uint64, replay bool) error {
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
	conR.newCommittee = nc
	return nil
}

func (conR *ConsensusReactor) NewCommitteeCleanup() {
	// clean up received map
	for _, nc := range conR.rcvdNewCommittee {
		delete(conR.rcvdNewCommittee, NewCommitteeKey{nc.KblockHeight, nc.Round})
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
	if conR.newCommittee.TimeoutTimer != nil {
		conR.newCommittee.TimeoutTimer.Stop()
		conR.newCommittee.TimeoutTimer = nil
	}
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
func (conR *ConsensusReactor) sendNewCommitteeMessage(peer *ConsensusPeer, leaderPubKey ecdsa.PublicKey, kblockHeight uint64, nonce uint64, round uint64) error {
	conR.updateCurEpoch(conR.chain.BestBlock().QC.EpochID)
	msg := &NewCommitteeMessage{
		CSMsgCommonHeader: ConsensusMsgCommonHeader{
			Height:    conR.curHeight,
			Round:     int(round),
			Sender:    crypto.FromECDSAPub(&conR.myPubKey),
			Timestamp: time.Now(),
			MsgType:   CONSENSUS_MSG_NEW_COMMITTEE,
			EpochID:   conR.curEpoch,
		},

		NextEpochID:    conR.curEpoch + 1,
		NewLeaderID:    crypto.FromECDSAPub(&leaderPubKey),
		ValidatorID:    crypto.FromECDSAPub(&conR.myPubKey),
		ValidatorBlsPK: conR.csCommon.GetSystem().PubKeyToBytes(*conR.csCommon.GetPublicKey()),
		Nonce:          nonce,
		KBlockHeight:   kblockHeight,
	}

	// sign message with bls key
	signMsg := conR.BuildNewCommitteeSignMsg(leaderPubKey, conR.curEpoch+1, uint64(conR.curHeight))
	blsSig, msgHash := conR.csCommon.SignMessage2([]byte(signMsg))
	msg.BlsSignature = blsSig
	msg.SignedMsgHash = msgHash

	// sign message with ecdsa key
	ecdsaSigBytes, err := conR.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		conR.logger.Error("Sign message failed", "error", err)
		return err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(ecdsaSigBytes)

	// state to init & send move to next round
	// fmt.Println("msg: %v", msg.String())
	var m ConsensusMessage = msg
	conR.asyncSendCommitteeMsg(&m, peer)
	return nil
}

// once reach 2/3 send aout annouce message
func (conR *ConsensusReactor) ProcessNewCommitteeMessage(newCommitteeMsg *NewCommitteeMessage, src *ConsensusPeer) bool {
	conR.logger.Info("received newCommittee Message", "source", src.name, "IP", src.netAddr.IP)
	ch := newCommitteeMsg.CSMsgCommonHeader

	// non replay case, last block must be kblock
	if conR.newCommittee.Replay == false && ch.Height != int64(newCommitteeMsg.KBlockHeight) {
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
	round := uint64(ch.Round)
	nonce := newCommitteeMsg.Nonce

	nc, ok := conR.rcvdNewCommittee[NewCommitteeKey{height, round}]
	if ok == false {
		nc = newNewCommittee(height, round, nonce)
		committee, role, index, inCommittee := conR.CalcCommitteeByNonce(conR.newCommittee.Nonce)
		nc.Committee = committee
		nc.Role = role
		nc.Index = index
		nc.InCommittee = inCommittee
		nc.sigAggregator = newSignatureAggregator(conR.committeeSize, conR.csCommon.system, msgHash, nc.Committee.Validators)

		conR.rcvdNewCommittee[NewCommitteeKey{height, round}] = nc
	} else {
		// same height round NewCommittee is there
		if nc.Nonce != nonce {
			conR.logger.Error("nonce mismtach between message and reactor", "recevied", nonce, "have", nc.Nonce)
			return false
		}
	}

	// check BLS key
	var index int
	var validator *types.Validator
	for i := range nc.Committee.Validators {
		v := nc.Committee.Validators[i]
		pubkey := crypto.FromECDSAPub(&v.PubKey)
		if bytes.Equal(pubkey, newCommitteeMsg.ValidatorID) == true {
			validator = v
			index = i
			break
		}
	}
	if validator == nil {
		conR.logger.Error("not find the validator in committee", "validator", validatorID, "nonce", nonce)
		return false
	}

	if bytes.Equal(conR.csCommon.GetSystem().PubKeyToBytes(validator.BlsPubKey), newCommitteeMsg.ValidatorBlsPK) == false {
		blsPK := base64.StdEncoding.EncodeToString(conR.csCommon.system.PubKeyToBytes(validator.BlsPubKey))
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
	if LeaderMajorityTwoThird(int(nc.sigAggregator.Count()), conR.committeeSize) {
		conR.logger.Debug("NewCommitteeMessage, 2/3 Majority reached", "Recvd", nc.sigAggregator.Count(), "committeeSize", conR.committeeSize, "replay", conR.newCommittee.Replay)

		// seal the signature, avoid re-trigger
		nc.sigAggregator.Seal()

		// Now it's time schedule leader
		if conR.csPacemaker.IsStopped() == false {
			conR.csPacemaker.Stop()
		}

		// nc.voterNum = 0
		aggSigBytes := nc.sigAggregator.Aggregate()
		aggSig, _ := conR.csCommon.system.SigFromBytes(aggSigBytes)

		// aggregate the signatures
		// nc.voterAggSig = conR.csCommon.AggregateSign(nc.voterSig)

		if conR.newCommittee.Replay == true {
			conR.ScheduleReplayLeader(epochID, NewNCEvidence(nc.sigAggregator.bitArray, nc.sigAggregator.msgHash, aggSig), WHOLE_NETWORK_BLOCK_SYNC_TIME)
		} else {
			// Wait for block sync since there is no time out yet
			conR.ScheduleLeader(epochID, height,
				NewNCEvidence(nc.sigAggregator.bitArray, nc.sigAggregator.msgHash, aggSig),
				WHOLE_NETWORK_BLOCK_SYNC_TIME)
		}

		conR.logger.Info("Leader scheduled ...")
		return true
	} else {
		// not reach 2/3 yet, wait for more
		conR.logger.Debug("received NewCommitteeMessage (2/3 not reached yet, wait for more)", "Recvd", nc.sigAggregator.Count(), "committeeSize", conR.committeeSize)
		return true
	}

}
