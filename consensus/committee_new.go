package consensus

import (
	"crypto/ecdsa"
	"time"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	types "github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	NEW_COMMITTEE_INIT_INTV = 15 * time.Second //15s
)

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

	// signature
	newCommitteeVoterBitArray *cmn.BitArray
	newCommitteeVoterSig      []bls.Signature
	newCommitteeVoterPubKey   []bls.PublicKey
	newCommitteeVoterMsgHash  [][32]byte
	newCommitteeVoterAggSig   bls.Signature
	newCommitteeVoterNum      int

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
func (conR *ConsensusReactor) sendNewCommitteeMessage(peer *ConsensusPeer, pubKey ecdsa.PublicKey, kblockHeight uint64, nonce uint64, round uint64) error {
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

		NewEpochID:      conR.curEpoch + 1,
		NewLeaderPubKey: crypto.FromECDSAPub(&pubKey),
		ValidatorPubkey: crypto.FromECDSAPub(&conR.myPubKey),
		Nonce:           nonce,
		KBlockHeight:    kblockHeight,

		// Signature part.
	}

	// sign message
	msgSig, err := conR.SignConsensusMsg(msg.SigningHash().Bytes())
	if err != nil {
		conR.logger.Error("Sign message failed", "error", err)
		return err
	}
	msg.CSMsgCommonHeader.SetMsgSignature(msgSig)

	// state to init & send move to next round
	// fmt.Println("msg: %v", msg.String())
	var m ConsensusMessage = msg
	conR.sendConsensusMsg(&m, peer)
	return nil
}

// once reach 2/3 send aout annouce message
func (conR *ConsensusReactor) ProcessNewCommitteeMessage(newCommitteeMsg *NewCommitteeMessage, src *ConsensusPeer) bool {

	ch := newCommitteeMsg.CSMsgCommonHeader

	if conR.ValidateCMheaderSig(&ch, newCommitteeMsg.SigningHash().Bytes()) == false {
		conR.logger.Error("Signature validate failed")
		return false
	}

	if ch.MsgType != CONSENSUS_MSG_NEW_COMMITTEE {
		conR.logger.Error("MsgType is not CONSENSUS_MSG_NEW_COMMITTEE")
		return false
	}

	// non replay case, last block must be kblock
	if conR.newCommittee.Replay == false && ch.Height != int64(newCommitteeMsg.KBlockHeight) {
		conR.logger.Error("CurHeight is not the same with kblock height")
		return false
	}

	// XXX: 1) Validator == sender 2) NewLeader == myself

	// sanity check done
	epochID := newCommitteeMsg.NewEpochID
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

		conR.rcvdNewCommittee[NewCommitteeKey{height, round}] = nc
	} else {
		// same height round NewCommittee is there
		if nc.Nonce != nonce {
			conR.logger.Error("nonce mismtach between message and reactor", "recevied", nonce, "have", nc.Nonce)
			return false
		}
	}

	//save bls signature
	nc.newCommitteeVoterNum++

	// 3. if the totoal vote > 2/3, move to Commit state
	if LeaderMajorityTwoThird(nc.newCommitteeVoterNum, conR.committeeSize) {
		conR.logger.Debug("NewCommitteeMessage, 2/3 Majority reached", "Recvd", nc.newCommitteeVoterNum, "committeeSize", conR.committeeSize, "replay", conR.newCommittee.Replay)

		// Now it's time schedule leader
		if conR.csPacemaker.IsStopped() == false {
			conR.csPacemaker.Stop()
		}

		// fmt.Println("replay", conR.newCommittee.Replay)
		if conR.newCommittee.Replay == true {
			conR.ScheduleReplayLeader(epochID, WHOLE_NETWORK_BLOCK_SYNC_TIME)
		} else {
			// Wait for block sync since there is no time out yet
			conR.ScheduleLeader(epochID, height, WHOLE_NETWORK_BLOCK_SYNC_TIME)
		}

		// avoid the re-trigger
		nc.newCommitteeVoterNum = 0
		return true
	} else {
		// not reach 2/3 yet, wait for more
		conR.logger.Debug("received NewCommitteeMessage (2/3 not reached yet, wait for more)", "Recvd", nc.newCommitteeVoterNum, "committeeSize", conR.committeeSize)
		return true
	}

}
