// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	cli "gopkg.in/urfave/cli.v1"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/comm"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/genesis"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/types"
)

const (
	//maxMsgSize = 1048576 // 1MB;
	// set as 1184 * 1024
	maxMsgSize = 1300000 // gasLimit 20000000 generate, 1024+1024 (1048576) + sizeof(QC) + sizeof(committee)...

	CHAN_DEFAULT_BUF_SIZE = 100
)

const (
	fromDelegatesFile = iota
	fromStaking
)

var (
	ConsensusGlobInst *ConsensusReactor
)

type ReactorConfig struct {
	InitCfgdDelegates bool
	EpochMBlockCount  uint32
	MinCommitteeSize  int
	MaxCommitteeSize  int
	MaxDelegateSize   int
	InitDelegates     []*types.Delegate
}

// -----------------------------------------------------------------------------
// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	logger       log15.Logger
	config       ReactorConfig
	SyncDone     bool

	// copy of master/node
	myPubKey  ecdsa.PublicKey  // this is my public identification !!
	myPrivKey ecdsa.PrivateKey // copy of private key

	// still references above consensuStae, reactor if this node is
	// involved the consensus
	delegateSize       int // global constant, current available delegate size.
	committeeSize      uint32
	curDelegates       *types.DelegateSet      // current delegates list
	curCommittee       *types.ValidatorSet     // This is top 400 of delegates by given nonce
	curActualCommittee []types.CommitteeMember // Real committee, should be subset of curCommittee if someone is offline.
	curCommitteeIndex  uint32

	csCommon    *types.ConsensusCommon //this must be allocated as validator
	csPacemaker *Pacemaker

	lastKBlockHeight   uint32
	curNonce           uint64
	curEpoch           uint64
	curHeight          uint32              // come from parentBlockID first 4 bytes uint32
	RcvKBlockInfoQueue chan RecvKBlockInfo // this channel for kblock notify from node module.

	magic           [4]byte
	inCommittee     bool
	allDelegates    []*types.Delegate
	sourceDelegates int

	msgCache *MsgCache
}

// Glob Instance
func GetConsensusGlobInst() *ConsensusReactor {
	return ConsensusGlobInst
}

func SetConsensusGlobInst(inst *ConsensusReactor) {
	ConsensusGlobInst = inst
}

// NewConsensusReactor returns a new ConsensusReactor with config
func NewConsensusReactor(ctx *cli.Context, chain *chain.Chain, state *state.Creator, privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey, magic [4]byte, blsCommon *BlsCommon, initDelegates []*types.Delegate) *ConsensusReactor {
	conR := &ConsensusReactor{
		chain:        chain,
		stateCreator: state,
		logger:       log15.New("pkg", "reactor"),
		SyncDone:     false,
		magic:        magic,
		inCommittee:  false,

		msgCache: NewMsgCache(1024),
	}

	if ctx != nil {
		conR.config = ReactorConfig{
			InitCfgdDelegates: ctx.Bool("init-configured-delegates"),
			EpochMBlockCount:  uint32(ctx.Uint("epoch-mblock-count")),
			MinCommitteeSize:  ctx.Int("committee-min-size"),
			MaxCommitteeSize:  ctx.Int("committee-max-size"),
			MaxDelegateSize:   ctx.Int("delegate-max-size"),
			InitDelegates:     initDelegates,
		}
	}
	// add the hardcoded genesis nonce in the case every node in block 0
	conR.RcvKBlockInfoQueue = make(chan RecvKBlockInfo, CHAN_DEFAULT_BUF_SIZE)

	//initialize height/round
	if chain.BestBlock().IsKBlock() {
		conR.lastKBlockHeight = chain.BestBlock().Number()
	} else {
		conR.lastKBlockHeight = chain.BestBlock().LastKBlockHeight()
	}
	lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))
	conR.curHeight = chain.BestBlock().Number()

	// initialize consensus common
	conR.csCommon = NewConsensusCommonFromBlsCommon(blsCommon)

	// initialize pacemaker
	conR.csPacemaker = NewPaceMaker(conR)

	// committee info is stored in the first of Mblock after Kblock
	if conR.curHeight != 0 {
		conR.updateCurEpoch(chain.BestBlock().GetBlockEpoch())
	} else {
		conR.curEpoch = 0
		curEpochGauge.Set(float64(0))
	}

	prometheus.Register(pmRoundGauge)
	prometheus.Register(curEpochGauge)
	prometheus.Register(lastKBlockHeightGauge)
	prometheus.Register(blocksCommitedCounter)
	prometheus.Register(inCommitteeGauge)
	prometheus.Register(pmRoleGauge)

	conR.myPrivKey = *privKey
	conR.myPubKey = *pubKey

	SetConsensusGlobInst(conR)
	return conR
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (conR *ConsensusReactor) OnStart() error {
	communicator := comm.GetGlobCommInst()
	if communicator == nil {
		conR.logger.Error("get communicator instance failed ...")
		return errors.New("could not get communicator")
	}
	select {
	case <-communicator.Synced():
		conR.SwitchToConsensus()
	}

	return nil
}

func (conR *ConsensusReactor) GetLastKBlockHeight() uint32 {
	return conR.lastKBlockHeight
}

// SwitchToConsensus switches from fast_sync mode to consensus mode.
// It resets the state, turns off fast_sync, and starts the consensus state-machine
func (conR *ConsensusReactor) SwitchToConsensus() {
	conR.logger.Info("sync is done, switch to consensus ...", "curHeight", conR.curHeight)

	conR.PrepareEnvForPacemaker()
	if conR.inCommittee {
		conR.startPacemaker(PMModeNormal)
	} else {
		conR.startPacemaker(PMModeObserve)
	}
}

func (conR *ConsensusReactor) UpdateHeight(height uint32) bool {
	if height != conR.curHeight {
		conR.logger.Debug(fmt.Sprintf("Update curHeight from %d to %d", conR.curHeight, height))
	}
	conR.curHeight = height
	return true
}

// update the LastKBlockHeight
// func (conR *ConsensusReactor) UpdateLastKBlockHeight(height uint32) bool {
// 	if height > conR.lastKBlockHeight {
// 		conR.lastKBlockHeight = height
// 		lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))
// 	}
// 	return true
// }

// Refresh the current Height from the best block
// normally call this routine after block chain changed
func (conR *ConsensusReactor) RefreshCurHeight() error {
	prev := conR.curHeight

	best := conR.chain.BestBlock()
	conR.curHeight = best.Number()
	if best.LastKBlockHeight() > conR.lastKBlockHeight {
		conR.lastKBlockHeight = best.LastKBlockHeight()
		lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))
	}
	conR.updateCurEpoch(best.GetBlockEpoch())

	lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))
	if prev != best.Number() {
		// conR.logger.Info("Refresh curHeight", "from", prev, "to", conR.curHeight, "lastKBlock", conR.lastKBlockHeight, "epoch", conR.curEpoch)
	}
	return nil
}

// actual committee is exactly the same as committee
// with one more field: CSIndex, the index order in committee
func (conR *ConsensusReactor) UpdateActualCommittee() bool {
	validators := make([]*types.Validator, 0)
	validators = append(validators, conR.curCommittee.Validators...)
	committee := make([]types.CommitteeMember, 0)
	for i, v := range validators {
		cm := types.CommitteeMember{
			Name:     v.Name,
			PubKey:   v.PubKey,
			NetAddr:  v.NetAddr,
			CSPubKey: v.BlsPubKey,
			CSIndex:  i, // (i + int(leaderIndex)) % size,
		}
		committee = append(committee, cm)
	}

	conR.curActualCommittee = committee
	return true
}

// get the specific round proposer
func (conR *ConsensusReactor) getRoundProposer(round uint32) types.CommitteeMember {
	size := len(conR.curActualCommittee)
	if size == 0 {
		return types.CommitteeMember{}
	}
	return conR.curActualCommittee[int(round)%size]
}

func (conR *ConsensusReactor) amIRoundProproser(round uint32) bool {
	p := conR.getRoundProposer(round)
	return bytes.Equal(crypto.FromECDSAPub(&p.PubKey), crypto.FromECDSAPub(&conR.myPubKey))
}

func (conR *ConsensusReactor) VerifyBothPubKey() {
	for _, d := range conR.curDelegates.Delegates {
		if bytes.Equal(crypto.FromECDSAPub(&d.PubKey), crypto.FromECDSAPub(&conR.myPubKey)) == true {
			if conR.GetCombinePubKey() != d.GetInternCombinePubKey() {
				fmt.Println("Combine PubKey: ", conR.GetCombinePubKey())
				fmt.Println("Intern Combine PubKey: ", d.GetInternCombinePubKey())
				panic("ECDSA key found in delegate list, but combinePubKey mismatch")
			}

			csCommonSystem := conR.csCommon.GetSystem()

			myBlsPubKey := csCommonSystem.PubKeyToBytes(conR.csCommon.PubKey)
			delegateBlsPubKey := csCommonSystem.PubKeyToBytes(d.BlsPubKey)

			if bytes.Equal(myBlsPubKey, delegateBlsPubKey) == false {
				panic("ECDSA key found in delegate list, but related BLS key mismatch with delegate, probably wrong info in candidate")
			}
		}
	}
}

// create validatorSet by a given nonce. return by my self role
func (conR *ConsensusReactor) UpdateCurCommitteeByNonce(nonce uint64) bool {
	committee, index, inCommittee := conR.calcCommitteeByNonce(nonce)
	conR.curCommittee = committee
	if inCommittee {
		conR.curCommitteeIndex = uint32(index)
		myAddr := conR.curCommittee.Validators[index].NetAddr
		myName := conR.curCommittee.Validators[index].Name
		conR.logger.Info("New committee calculated", "index", index, "myName", myName, "myIP", myAddr.IP.String())
	} else {
		conR.curCommitteeIndex = 0
		// FIXME: find a better way
		conR.logger.Info("New committee calculated")
	}
	fmt.Println("Committee members in order:")
	fmt.Println(committee)

	return inCommittee
}

// it is used for temp calculate committee set by a given nonce in the fly.
// also return the committee
func (conR *ConsensusReactor) calcCommitteeByNonce(nonce uint64) (*types.ValidatorSet, int, bool) {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)

	vals := make([]*types.Validator, 0)
	for _, d := range conR.curDelegates.Delegates {
		v := &types.Validator{
			Name:        string(d.Name),
			Address:     d.Address,
			PubKey:      d.PubKey,
			BlsPubKey:   d.BlsPubKey,
			VotingPower: d.VotingPower,
			NetAddr:     d.NetAddr,
			CommitKey:   crypto.Keccak256(append(crypto.FromECDSAPub(&d.PubKey), buf...)),
		}
		vals = append(vals, v)
	}

	sort.SliceStable(vals, func(i, j int) bool {
		return (bytes.Compare(vals[i].CommitKey, vals[j].CommitKey) <= 0)
	})

	vals = vals[:conR.committeeSize]
	// the full list is stored in currCommittee, sorted.
	// To become a validator (real member in committee), must repond the leader's
	// announce. Validators are stored in conR.conS.Vlidators
	Committee := types.NewValidatorSet2(vals)
	if len(vals) < 1 {
		conR.logger.Error("VALIDATOR SET is empty, potential error config with delegates.json", "delegates", len(conR.curDelegates.Delegates))

		return Committee, 0, false
	}

	if bytes.Equal(crypto.FromECDSAPub(&vals[0].PubKey), crypto.FromECDSAPub(&conR.myPubKey)) {
		return Committee, 0, true
	}

	for i, val := range vals {
		if bytes.Equal(crypto.FromECDSAPub(&val.PubKey), crypto.FromECDSAPub(&conR.myPubKey)) {

			return Committee, i, true
		}
	}

	return Committee, 0, false
}

func (conR *ConsensusReactor) GetCommitteeMemberIndex(pubKey ecdsa.PublicKey) int {
	for i, v := range conR.curCommittee.Validators {
		if bytes.Equal(crypto.FromECDSAPub(&v.PubKey), crypto.FromECDSAPub(&pubKey)) {
			return i
		}
	}
	conR.logger.Error("I'm not in committee, please check public key settings", "pubKey", pubKey)
	return -1
}

func (conR *ConsensusReactor) GetMyNetAddr() types.NetAddress {
	if conR.curCommittee != nil && len(conR.curCommittee.Validators) > 0 {
		return conR.curCommittee.Validators[conR.curCommitteeIndex].NetAddr
	}
	return types.NetAddress{IP: net.IP{}, Port: 0}
}

func (conR *ConsensusReactor) GetMyName() string {
	if conR.curCommittee != nil && len(conR.curCommittee.Validators) > 0 {
		return conR.curCommittee.Validators[conR.curCommitteeIndex].Name
	}
	return "unknown"
}

func (conR *ConsensusReactor) GetMyActualCommitteeIndex() int {
	myNetAddr := conR.GetMyNetAddr()
	for index, member := range conR.curActualCommittee {
		if member.NetAddr.IP.String() == myNetAddr.IP.String() {
			return index
		}
	}
	return -1
}

// update current delegates with new delegates from staking or config file
// keep this standalone method intentionly
func (conR *ConsensusReactor) UpdateCurDelegates() {
	delegates, delegateSize, committeeSize := conR.GetConsensusDelegates()
	conR.allDelegates = delegates
	conR.curDelegates = types.NewDelegateSet(delegates[:delegateSize])
	conR.delegateSize = delegateSize
	conR.committeeSize = uint32(committeeSize)
}

func (conR *ConsensusReactor) PrepareEnvForPacemaker() error {
	var nonce uint64
	var info *powpool.PowBlockInfo
	bestKBlock, err := conR.chain.BestKBlock()
	if err != nil {
		fmt.Println("could not get best KBlock", err)
		return errors.New("could not get best KBlock")
	}
	bestBlock := conR.chain.BestBlock()
	bestIsKBlock := (bestBlock.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK) || bestBlock.Header().Number() == 0
	kBlockHeight := bestKBlock.Header().Number()

	epoch := uint64(0)
	if kBlockHeight == 0 {
		nonce = genesis.GenesisNonce
		info = powpool.GetPowGenesisBlockInfo()
	} else {
		nonce = bestKBlock.KBlockData.Nonce
		info = powpool.NewPowBlockInfoFromPosKBlock(bestKBlock)
		epoch = bestKBlock.GetBlockEpoch() + 1
	}

	if conR.curNonce == nonce {
		conR.logger.Info("committee initialized already", "nonce", nonce, "kBlock", kBlockHeight, "bestIsKBlock", bestIsKBlock, "epoch", epoch)
		return nil
	}

	conR.logger.Info("prepare env for pacemaker", "nonce", nonce, "kBlock", kBlockHeight, "bestIsKBlock", bestIsKBlock, "epoch", epoch)

	//initialize Delegates
	conR.UpdateCurDelegates()

	// notice: this will panic if ECDSA key matches but BLS doesn't
	conR.VerifyBothPubKey()

	conR.curNonce = nonce
	inCommittee := conR.UpdateCurCommitteeByNonce(nonce)
	conR.inCommittee = inCommittee

	conR.lastKBlockHeight = kBlockHeight
	lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))
	conR.updateCurEpoch(epoch)
	conR.UpdateActualCommittee()

	conR.logger.Info("Powpool prepare to add kframe, and notify PoW chain to pick head", "powHeight", info.PowHeight, "powRawBlock", hex.EncodeToString(info.PowRaw))
	pool := powpool.GetGlobPowPoolInst()
	// pool.Wash()
	pool.InitialAddKframe(info)
	conR.logger.Debug("Powpool initial added kframe", "bestKblock", kBlockHeight, "powHeight", info.PowHeight)

	if inCommittee {
		conR.logger.Info("I am in committee!!!")
		if bestIsKBlock {
			//kblock is already added to pool, should start with next one
			startHeight := info.PowHeight + 1
			conR.logger.Info("Replay pow blocks", "fromHeight", startHeight)
			pool.ReplayFrom(int32(startHeight))
		}
		conR.inCommittee = true
		inCommitteeGauge.Set(1)

		// verify in committee status with consent block (1st mblock in epoch)
		// if delegates are obtained from staking
		if !conR.config.InitCfgdDelegates && !bestIsKBlock {
			consentBlock, err := conR.chain.GetTrunkBlock(kBlockHeight + 1)
			if err != nil {
				fmt.Println("could not get committee info:", err)
				return errors.New("could not get committee info for status check")
			}
			// recover actual committee from consent block
			committeeInfo := consentBlock.CommitteeInfos
			// leaderIndex := committeeInfo.CommitteeInfo[0].CSIndex
			conR.UpdateActualCommittee()

			myself := conR.curCommittee.Validators[conR.curCommitteeIndex]
			myEcdsaPKBytes := crypto.FromECDSAPub(&myself.PubKey)
			inCommitteeVerified := false
			for _, v := range committeeInfo.CommitteeInfo {
				if bytes.Compare(v.PubKey, myEcdsaPKBytes) == 0 {
					inCommitteeVerified = true
					break
				}
			}
			if !inCommitteeVerified {
				conR.logger.Error("committee info in consent block doesn't contain myself as a member, stop right now")
				return errors.New("committee info in consent block doesnt match myself")
			}
		}

	} else {
		conR.inCommittee = false
		inCommitteeGauge.Set(0)
		conR.logger.Info("I am NOT in committee!!!", "nonce", nonce)
	}
	return nil
}

func (conR *ConsensusReactor) verifyBestQCAndBestBlockBeforeStart() bool {
	// 1. bestQC height == best block height
	// 2. newCommittee is true, best block is kblock
	for i := 0; i < 3; i++ {
		conR.chain.UpdateBestQC(nil, chain.None)
		bestQC := conR.chain.BestQC()
		bestBlock := conR.chain.BestBlock()
		// conR.logger.Info("Checking the QCHeight and Block height...", "QCHeight", bestQC.QCHeight, "bestHeight", bestBlock.Number())
		if bestQC.QCHeight != bestBlock.Number() {
			com := comm.GetGlobCommInst()
			if com == nil {
				conR.logger.Error("get global comm inst failed")
				return false
			}
			conR.logger.Warn("bestQC and bestBlock mismatch, trigger sync now ...", "bestQC", bestQC.QCHeight, "bestBlock", bestBlock.Number(), "attempt", i+1, "waitInterval", time.Duration(math.Pow(float64(2), float64(i+1))))
			com.TriggerSync()
			// every attempt wait for 2^(i+1) seconds
			<-time.NewTimer(time.Duration(math.Pow(float64(2), float64(i+1))) * time.Second).C
		} else {
			break
		}
	}

	conR.chain.UpdateBestQC(nil, chain.None)
	bestQC := conR.chain.BestQC()
	bestBlock := conR.chain.BestBlock()
	if bestQC.QCHeight != bestBlock.Number() {
		conR.logger.Warn("Caution: bestQC and bestBlock mismatch after syncing ...", "bestQC", bestQC.QCHeight, "bestBlock", bestBlock.Number())
		return false
	} else {
		conR.logger.Info("bestQC and bestBlock matches", "bestQC", bestQC.QCHeight, "bestBlock", bestBlock.Number())
		return true
	}
}

func (conR *ConsensusReactor) startPacemaker(mode PMMode) error {
	// 1. bestQC height == best block height
	// 2. newCommittee is true, best block is kblock
	verified := conR.verifyBestQCAndBestBlockBeforeStart()
	// bestQC := conR.chain.BestQC()
	bestBlock := conR.chain.BestBlock()
	freshCommittee := (bestBlock.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK) || (bestBlock.Header().Number() == 0)
	if verified {
		// conR.logger.Info("start Pacemaker", "bestQC", bestQC.QCHeight, "bestBlock", bestBlock.Header().Number())
		conR.csPacemaker.Start(mode, freshCommittee)
	} else {
		// conR.logger.Warn("start Pacemaker in CatchUp mode due to bestQC/bestBlock mismatch", "bestQC", bestQC.QCHeight, "bestBlock", bestBlock.Header().Number())
		conR.csPacemaker.Start(PMModeCatchUp, freshCommittee)
	}
	return nil
}

// since votes of pacemaker include propser, but committee votes
// do not have leader itself, we seperate the majority func
// Easier adjust the logic of major 2/3, for pacemaker
func MajorityTwoThird(voterNum, committeeSize uint32) bool {
	if (voterNum < 0) || (committeeSize < 1) {
		fmt.Println("MajorityTwoThird, inputs out of range")
		return false
	}
	// Examples
	// committeeSize= 1 twoThirds= 1
	// committeeSize= 2 twoThirds= 2
	// committeeSize= 3 twoThirds= 2
	// committeeSize= 4 twoThirds= 3
	// committeeSize= 5 twoThirds= 4
	// committeeSize= 6 twoThirds= 4
	twoThirds := math.Ceil(float64(committeeSize) * 2 / 3)
	if float64(voterNum) >= twoThirds {
		return true
	}
	return false
}

func (conR *ConsensusReactor) splitPubKey(comboPub string) (*ecdsa.PublicKey, *bls.PublicKey) {
	// first part is ecdsa public, 2nd part is bls public key
	split := strings.Split(comboPub, ":::")
	// fmt.Println("ecdsa PubKey", split[0], "Bls PubKey", split[1])
	pubKeyBytes, err := b64.StdEncoding.DecodeString(split[0])
	if err != nil {
		panic(fmt.Sprintf("read public key of delegate failed, %v", err))
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		panic(fmt.Sprintf("read public key of delegate failed, %v", err))
	}

	blsPubBytes, err := b64.StdEncoding.DecodeString(split[1])
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}
	blsPub, err := conR.csCommon.GetSystem().PubKeyFromBytes(blsPubBytes)
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}

	return pubKey, &blsPub
}

func (conR *ConsensusReactor) combinePubKey(ecdsaPub *ecdsa.PublicKey, blsPub *bls.PublicKey) string {
	ecdsaPubBytes := crypto.FromECDSAPub(ecdsaPub)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)

	blsPubBytes := conR.csCommon.GetSystem().PubKeyToBytes(*blsPub)
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	return strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
}

func (conR *ConsensusReactor) GetCombinePubKey() string {
	return conR.combinePubKey(&conR.myPubKey, &conR.csCommon.PubKey)
}

func (conR *ConsensusReactor) LoadBlockBytes(num uint32) []byte {
	raw, err := conR.chain.GetTrunkBlockRaw(num)
	if err != nil {
		log.Error("Error load raw block", "err", err)
		return []byte{}
	}
	return raw[:]
}

func calcCommitteeSize(delegateSize int, config ReactorConfig) (int, int) {
	if delegateSize >= config.MaxDelegateSize {
		delegateSize = config.MaxDelegateSize
	}

	committeeSize := delegateSize
	if delegateSize > config.MaxCommitteeSize {
		committeeSize = config.MaxCommitteeSize
	}
	return delegateSize, committeeSize
}

// entry point for each committee
// return with delegates list, delegateSize, committeeSize
// maxDelegateSize >= maxCommiteeSize >= minCommitteeSize
func (conR *ConsensusReactor) GetConsensusDelegates() ([]*types.Delegate, int, int) {
	forceDelegates := conR.config.InitCfgdDelegates

	// special handle for flag --init-configured-delegates
	var delegates []*types.Delegate
	var err error
	hint := ""
	if forceDelegates {
		delegates = conR.config.InitDelegates
		conR.sourceDelegates = fromDelegatesFile
		hint = "Loaded delegates from delegates.json"
	} else {
		delegates, err = conR.getDelegatesFromStaking()
		hint = "Loaded delegates from staking"
		conR.sourceDelegates = fromStaking
		if err != nil || len(delegates) < conR.config.MinCommitteeSize {
			delegates = conR.config.InitDelegates
			hint = "Loaded delegates from delegates.json as fallback, error loading staking candiates"
			conR.sourceDelegates = fromDelegatesFile
		}
	}

	delegateSize, committeeSize := calcCommitteeSize(len(delegates), conR.config)
	conR.allDelegates = delegates

	first3Names := make([]string, 0)
	if len(delegates) > 3 {
		for _, d := range delegates[:3] {
			name := string(d.Name)
			first3Names = append(first3Names, name)
		}
	} else {
		for _, d := range delegates {
			name := string(d.Name)
			first3Names = append(first3Names, name)
		}

	}

	conR.logger.Info(hint, "delegateSize", delegateSize, "committeeSize", committeeSize, "first3", strings.Join(first3Names, ","))
	// PrintDelegates(delegates[:delegateSize])
	return delegates, delegateSize, committeeSize
}

func (conR *ConsensusReactor) GetDelegateNameByIP(ip net.IP) string {
	for _, d := range conR.allDelegates {
		if d.NetAddr.IP.String() == ip.String() {
			return string(d.Name)
		}
	}

	return ""
}

func (conR *ConsensusReactor) updateCurEpoch(epoch uint64) {
	if epoch > conR.curEpoch {
		oldVal := conR.curEpoch
		conR.curEpoch = epoch
		curEpochGauge.Set(float64(conR.curEpoch))
		conR.logger.Info("Epoch updated", "from", oldVal, "to", conR.curEpoch)
	}
}

// ------------------------------------
// UTILITY
// ------------------------------------
func PrintDelegates(delegates []*types.Delegate) {
	fmt.Println("============================================")
	for i, dd := range delegates {
		keyBytes := crypto.FromECDSAPub(&dd.PubKey)
		pubKeyStr := base64.StdEncoding.EncodeToString(keyBytes)
		pubKeyAbbr := pubKeyStr[:4] + "..." + pubKeyStr[len(pubKeyStr)-4:]
		fmt.Printf("#%d: %s (%s) :%d  Address:%s PubKey: %s Commission: %v%% #Dists: %v\n",
			i+1, dd.Name, dd.NetAddr.IP.String(), dd.NetAddr.Port, dd.Address, pubKeyAbbr, dd.Commission/1e7, len(dd.DistList))
	}
	fmt.Println("============================================")
}

func (conR *ConsensusReactor) OnReceiveMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		conR.logger.Error("Unrecognized payload", "err", err)
		return
	}
	mi, err := conR.UnmarshalMsg(data)
	if err != nil {
		conR.logger.Error("Unmarshal error", "err", err)
		return
	}

	msg, peer := mi.Msg, mi.Peer
	typeName := mi.Msg.GetType()

	signerIndex := msg.GetSignerIndex()
	signer := conR.curActualCommittee[signerIndex]

	if msg.VerifyMsgSignature(&signer.PubKey) == false {
		conR.logger.Error("invalid signature, dropped ...", "peer", peer.NameAndIP(), "msg", msg.String(), "signer", signer.Name)
		return
	}
	mi.Signer = signer

	// sanity check for PMProposal
	if msg.GetType() == "PMProposal" {
		proposalMsg := msg.(*PMProposalMessage)
		blk := proposalMsg.DecodeBlock()
		if blk == nil {
			conR.logger.Error("Invalid PMProposal: could not decode proposed block")
			return
		}
		if blk.Number() != proposalMsg.Height {
			conR.logger.Error("Invalid PMProposal: block.number != msg.height")
			return
		}
	}

	if msg.GetType() == "PMTimeout" {
		timeoutMsg := msg.(*PMTimeoutMessage)
		qcHigh := timeoutMsg.DecodeQCHigh()
		if qcHigh == nil {
			conR.logger.Error("Invalid QCHigh: could not decode qcHigh")
		}
	}

	fromMyself := peer.netAddr.IP.String() == conR.GetMyNetAddr().IP.String()
	suffix := ""
	if fromMyself {
		suffix = " (myself)"
	}

	if msg.GetEpoch() < conR.curEpoch {
		conR.logger.Info(fmt.Sprintf("recv outdated %s", msg.String()), "peer", peer.NameAndIP()+suffix)
		return
	}

	if conR.csPacemaker == nil {
		conR.logger.Warn("pacemaker is not initialized, dropped message")
		return
	}
	conR.csPacemaker.onIncomingMsg(mi)

	// relay the message if these two conditions are met:
	// 1. the original message is not sent by myself
	// 2. it's a proposal message
	if fromMyself == false && typeName == "PMProposal" {
		conR.relayMsg(mi)
	}
}

// Assumptions to use this:
// myIndex is always 0 for proposer (or leader in consensus)
// indexes starts from 0
// 1st layer: 				0  (proposer)
// 2nd layer: 				[1, 2], [3, 4], [5, 6], [7, 8]
// 3rd layer (32 groups):   [9..] ...
func CalcRelayPeers(myIndex, size int) (peers []int) {
	peers = []int{}
	if myIndex > size {
		fmt.Println("Input wrong!!! myIndex > size")
		return
	}
	replicas := 8
	if size <= replicas {
		replicas = size
		for i := 1; i <= replicas; i++ {
			peers = append(peers, (myIndex+i)%size)
		}
	} else {
		replica1 := 8
		replica2 := 4
		replica3 := 2
		limit1 := int(math.Ceil(float64(size)/float64(replica1))) * 2
		limit2 := limit1 + int(math.Ceil(float64(size)/float64(replica2)))*4

		if myIndex < limit1 {
			base := myIndex * replica1
			for i := 1; i <= replica1; i++ {
				peers = append(peers, (base+i)%size)
			}
		} else if myIndex >= limit1 && myIndex < limit2 {
			base := replica1*limit1 + (myIndex-limit1)*replica2
			for i := 1; i <= replica2; i++ {
				peers = append(peers, (base+i)%size)
			}
		} else if myIndex >= limit2 {
			base := replica1*limit1 + (limit2-limit1)*replica2 + (myIndex-limit2)*replica3
			for i := 1; i <= replica3; i++ {
				peers = append(peers, (base+i)%size)
			}
		}
	}
	return

	// if myIndex == 0 {
	// 	var k int
	// 	if maxIndex >= 8 {
	// 		k = 8
	// 	} else {
	// 		k = maxIndex
	// 	}
	// 	for i := 1; i <= k; i++ {
	// 		peers = append(peers, i)
	// 	}
	// 	return
	// }
	// if maxIndex <= 8 {
	// 	return //no peer
	// }

	// var groupSize, groupCount int
	// groupSize = ((maxIndex - 8) / 16) + 1
	// groupCount = (maxIndex - 8) / groupSize
	// // fmt.Println("groupSize", groupSize, "groupCount", groupCount)

	// if myIndex <= 8 {
	// 	mySet := (myIndex - 1) / 4
	// 	myRole := (myIndex - 1) % 4
	// 	for i := 0; i < 8; i++ {
	// 		group := (mySet * 8) + i
	// 		if group >= groupCount {
	// 			return
	// 		}

	// 		begin := 9 + (group * groupSize)
	// 		if myRole == 0 {
	// 			peers = append(peers, begin)
	// 		} else {
	// 			end := begin + groupSize - 1
	// 			if end > maxIndex {
	// 				end = maxIndex
	// 			}
	// 			middle := (begin + end) / 2
	// 			peers = append(peers, middle)
	// 		}
	// 	}
	// } else {
	// 	// I am in group, so begin << myIndex << end
	// 	// if wrap happens, redundant the 2nd layer
	// 	group := (maxIndex - 8) / 16
	// 	begin := 9 + (group * groupSize)
	// 	end := begin + groupSize - 1
	// 	if end > maxIndex {
	// 		end = maxIndex
	// 	}

	// 	var peerIndex int
	// 	var wrap bool = false
	// 	if myIndex == end && end != begin {
	// 		peers = append(peers, begin)
	// 	}
	// 	if peerIndex = myIndex + 1; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if peerIndex = myIndex + 2; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if peerIndex = myIndex + 4; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if peerIndex = myIndex + 8; peerIndex <= maxIndex {
	// 		peers = append(peers, peerIndex)
	// 	} else {
	// 		wrap = true
	// 	}
	// 	if wrap == true {
	// 		peers = append(peers, (myIndex%8)+1)
	// 		peers = append(peers, (myIndex%8)+1+8)
	// 	}
	// }
	// return
}
