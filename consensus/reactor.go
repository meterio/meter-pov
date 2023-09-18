// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"context"
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
	ConsensusGlobInst *Reactor
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
// Reactor defines a reactor for the consensus service.
type Reactor struct {
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

	blsCommon *types.BlsCommon //this must be allocated as validator
	pacemaker *Pacemaker

	lastKBlockHeight uint32
	curNonce         uint64
	curEpoch         uint64
	curHeight        uint32            // come from parentBlockID first 4 bytes uint32
	EpochEndCh       chan EpochEndInfo // this channel for kblock notify from node module.

	magic           [4]byte
	inCommittee     bool
	allDelegates    []*types.Delegate
	sourceDelegates int

	inQueue  *IncomingQueue
	outQueue *OutgoingQueue
}

// Glob Instance
func GetConsensusGlobInst() *Reactor {
	return ConsensusGlobInst
}

func SetConsensusGlobInst(inst *Reactor) {
	ConsensusGlobInst = inst
}

// NewConsensusReactor returns a new Reactor with config
func NewConsensusReactor(ctx *cli.Context, chain *chain.Chain, state *state.Creator, privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey, magic [4]byte, blsCommon *types.BlsCommon, initDelegates []*types.Delegate) *Reactor {
	r := &Reactor{
		chain:        chain,
		stateCreator: state,
		logger:       log15.New("pkg", "reactor"),
		SyncDone:     false,
		magic:        magic,
		inCommittee:  false,

		inQueue:  NewIncomingQueue(),
		outQueue: NewOutgoingQueue(),
	}

	if ctx != nil {
		r.config = ReactorConfig{
			InitCfgdDelegates: ctx.Bool("init-configured-delegates"),
			EpochMBlockCount:  uint32(ctx.Uint("epoch-mblock-count")),
			MinCommitteeSize:  ctx.Int("committee-min-size"),
			MaxCommitteeSize:  ctx.Int("committee-max-size"),
			MaxDelegateSize:   ctx.Int("delegate-max-size"),
			InitDelegates:     initDelegates,
		}
	}
	// add the hardcoded genesis nonce in the case every node in block 0
	r.EpochEndCh = make(chan EpochEndInfo, CHAN_DEFAULT_BUF_SIZE)

	//initialize height/round
	if chain.BestBlock().IsKBlock() {
		r.lastKBlockHeight = chain.BestBlock().Number()
	} else {
		r.lastKBlockHeight = chain.BestBlock().LastKBlockHeight()
	}
	lastKBlockHeightGauge.Set(float64(r.lastKBlockHeight))
	r.curHeight = chain.BestBlock().Number()

	// initialize consensus common
	r.blsCommon = blsCommon

	// initialize pacemaker
	r.pacemaker = NewPacemaker(r)

	// committee info is stored in the first of Mblock after Kblock
	if r.curHeight != 0 {
		r.updateCurEpoch(chain.BestBlock().GetBlockEpoch())
	} else {
		r.curEpoch = 0
		curEpochGauge.Set(float64(0))
	}

	prometheus.Register(pmRoundGauge)
	prometheus.Register(curEpochGauge)
	prometheus.Register(lastKBlockHeightGauge)
	prometheus.Register(blocksCommitedCounter)
	prometheus.Register(inCommitteeGauge)
	prometheus.Register(pmRoleGauge)

	r.myPrivKey = *privKey
	r.myPubKey = *pubKey

	SetConsensusGlobInst(r)
	return r
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (r *Reactor) OnStart(ctx context.Context) error {
	go r.outQueue.Start(ctx)
	communicator := comm.GetGlobCommInst()
	if communicator == nil {
		r.logger.Error("get communicator instance failed ...")
		return errors.New("could not get communicator")
	}
	select {
	case <-ctx.Done():
		r.logger.Warn("stop reactor due to context end")
		return nil
	case <-communicator.Synced():
		r.logger.Info("sync is done, start pacemaker ...", "curHeight", r.curHeight)
		r.pacemaker.Regulate()
	}

	return nil
}

func (r *Reactor) GetLastKBlockHeight() uint32 {
	return r.lastKBlockHeight
}

func (r *Reactor) UpdateHeight(height uint32) bool {
	if height != r.curHeight {
		r.logger.Debug(fmt.Sprintf("Update curHeight from %d to %d", r.curHeight, height))
	}
	r.curHeight = height
	return true
}

// update the LastKBlockHeight
// func (r *Reactor) UpdateLastKBlockHeight(height uint32) bool {
// 	if height > r.lastKBlockHeight {
// 		r.lastKBlockHeight = height
// 		lastKBlockHeightGauge.Set(float64(r.lastKBlockHeight))
// 	}
// 	return true
// }

// Refresh the current Height from the best block
// normally call this routine after block chain changed
func (r *Reactor) RefreshCurHeight() error {
	prev := r.curHeight

	best := r.chain.BestBlock()
	r.curHeight = best.Number()
	if best.LastKBlockHeight() > r.lastKBlockHeight {
		r.lastKBlockHeight = best.LastKBlockHeight()
		lastKBlockHeightGauge.Set(float64(r.lastKBlockHeight))
	}
	r.updateCurEpoch(best.GetBlockEpoch())

	lastKBlockHeightGauge.Set(float64(r.lastKBlockHeight))
	if prev != best.Number() {
		// r.logger.Info("Refresh curHeight", "from", prev, "to", r.curHeight, "lastKBlock", r.lastKBlockHeight, "epoch", r.curEpoch)
	}
	return nil
}

// actual committee is exactly the same as committee
// with one more field: CSIndex, the index order in committee
func (r *Reactor) UpdateActualCommittee() bool {
	validators := make([]*types.Validator, 0)
	validators = append(validators, r.curCommittee.Validators...)
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

	r.curActualCommittee = committee
	return true
}

// get the specific round proposer
func (r *Reactor) getRoundProposer(round uint32) types.CommitteeMember {
	size := len(r.curActualCommittee)
	if size == 0 {
		return types.CommitteeMember{}
	}
	return r.curActualCommittee[int(round)%size]
}

func (r *Reactor) amIRoundProproser(round uint32) bool {
	p := r.getRoundProposer(round)
	return bytes.Equal(crypto.FromECDSAPub(&p.PubKey), crypto.FromECDSAPub(&r.myPubKey))
}

func (r *Reactor) VerifyBothPubKey() {
	for _, d := range r.curDelegates.Delegates {
		if bytes.Equal(crypto.FromECDSAPub(&d.PubKey), crypto.FromECDSAPub(&r.myPubKey)) == true {
			if r.GetCombinePubKey() != d.GetInternCombinePubKey() {
				fmt.Println("Combine PubKey: ", r.GetCombinePubKey())
				fmt.Println("Intern Combine PubKey: ", d.GetInternCombinePubKey())
				panic("ECDSA key found in delegate list, but combinePubKey mismatch")
			}

			blsCommonSystem := r.blsCommon.GetSystem()

			myBlsPubKey := blsCommonSystem.PubKeyToBytes(r.blsCommon.PubKey)
			delegateBlsPubKey := blsCommonSystem.PubKeyToBytes(d.BlsPubKey)

			if bytes.Equal(myBlsPubKey, delegateBlsPubKey) == false {
				panic("ECDSA key found in delegate list, but related BLS key mismatch with delegate, probably wrong info in candidate")
			}
		}
	}
}

// create validatorSet by a given nonce. return by my self role
func (r *Reactor) UpdateCurCommitteeByNonce(nonce uint64) bool {
	committee, index, inCommittee := r.calcCommitteeByNonce(nonce)
	r.curCommittee = committee
	if inCommittee {
		r.curCommitteeIndex = uint32(index)
		myAddr := r.curCommittee.Validators[index].NetAddr
		myName := r.curCommittee.Validators[index].Name
		r.logger.Info("New committee calculated", "index", index, "myName", myName, "myIP", myAddr.IP.String())
	} else {
		r.curCommitteeIndex = 0
		// FIXME: find a better way
		r.logger.Info("New committee calculated")
	}
	fmt.Println("Committee members in order:")
	fmt.Println(committee)

	return inCommittee
}

// it is used for temp calculate committee set by a given nonce in the fly.
// also return the committee
func (r *Reactor) calcCommitteeByNonce(nonce uint64) (*types.ValidatorSet, int, bool) {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)

	vals := make([]*types.Validator, 0)
	for _, d := range r.curDelegates.Delegates {
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

	vals = vals[:r.committeeSize]
	// the full list is stored in currCommittee, sorted.
	// To become a validator (real member in committee), must repond the leader's
	// announce. Validators are stored in r.conS.Vlidators
	Committee := types.NewValidatorSet2(vals)
	if len(vals) < 1 {
		r.logger.Error("VALIDATOR SET is empty, potential error config with delegates.json", "delegates", len(r.curDelegates.Delegates))

		return Committee, 0, false
	}

	if bytes.Equal(crypto.FromECDSAPub(&vals[0].PubKey), crypto.FromECDSAPub(&r.myPubKey)) {
		return Committee, 0, true
	}

	for i, val := range vals {
		if bytes.Equal(crypto.FromECDSAPub(&val.PubKey), crypto.FromECDSAPub(&r.myPubKey)) {

			return Committee, i, true
		}
	}

	return Committee, 0, false
}

func (r *Reactor) GetCommitteeMemberIndex(pubKey ecdsa.PublicKey) int {
	for i, v := range r.curCommittee.Validators {
		if bytes.Equal(crypto.FromECDSAPub(&v.PubKey), crypto.FromECDSAPub(&pubKey)) {
			return i
		}
	}
	r.logger.Error("I'm not in committee, please check public key settings", "pubKey", pubKey)
	return -1
}

func (r *Reactor) GetMyNetAddr() types.NetAddress {
	if r.curCommittee != nil && len(r.curCommittee.Validators) > 0 {
		return r.curCommittee.Validators[r.curCommitteeIndex].NetAddr
	}
	return types.NetAddress{IP: net.IP{}, Port: 0}
}

func (r *Reactor) GetMyName() string {
	if r.curCommittee != nil && len(r.curCommittee.Validators) > 0 {
		return r.curCommittee.Validators[r.curCommitteeIndex].Name
	}
	return "unknown"
}

func (r *Reactor) GetMyActualCommitteeIndex() int {
	myNetAddr := r.GetMyNetAddr()
	for index, member := range r.curActualCommittee {
		if member.NetAddr.IP.String() == myNetAddr.IP.String() {
			return index
		}
	}
	return -1
}

// update current delegates with new delegates from staking or config file
// keep this standalone method intentionly
func (r *Reactor) UpdateCurDelegates() {
	delegates, delegateSize, committeeSize := r.GetConsensusDelegates()
	r.allDelegates = delegates
	r.curDelegates = types.NewDelegateSet(delegates[:delegateSize])
	r.delegateSize = delegateSize
	r.committeeSize = uint32(committeeSize)
}

func (r *Reactor) PrepareEnvForPacemaker() error {
	var nonce uint64
	var info *powpool.PowBlockInfo
	bestKBlock, err := r.chain.BestKBlock()
	if err != nil {
		fmt.Println("could not get best KBlock", err)
		return errors.New("could not get best KBlock")
	}
	bestBlock := r.chain.BestBlock()
	bestIsKBlock := (bestBlock.Header().BlockType() == block.KBlockType) || bestBlock.Header().Number() == 0
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

	if r.curNonce == nonce {
		r.logger.Info("committee initialized already", "nonce", nonce, "kBlock", kBlockHeight, "bestIsKBlock", bestIsKBlock, "epoch", epoch)
		return nil
	}

	r.logger.Info("prepare env for pacemaker", "nonce", nonce, "kBlock", kBlockHeight, "bestIsKBlock", bestIsKBlock, "epoch", epoch)

	//initialize Delegates
	r.UpdateCurDelegates()

	// notice: this will panic if ECDSA key matches but BLS doesn't
	r.VerifyBothPubKey()

	r.curNonce = nonce
	inCommittee := r.UpdateCurCommitteeByNonce(nonce)
	r.inCommittee = inCommittee

	r.lastKBlockHeight = kBlockHeight
	lastKBlockHeightGauge.Set(float64(r.lastKBlockHeight))
	r.updateCurEpoch(epoch)
	r.UpdateActualCommittee()

	r.logger.Info("Powpool prepare to add kframe, and notify PoW chain to pick head", "powHeight", info.PowHeight, "powRawBlock", hex.EncodeToString(info.PowRaw))
	pool := powpool.GetGlobPowPoolInst()
	// pool.Wash()
	pool.InitialAddKframe(info)
	r.logger.Debug("Powpool initial added kframe", "bestKblock", kBlockHeight, "powHeight", info.PowHeight)

	if inCommittee {
		r.logger.Info("I am in committee!!!")
		if bestIsKBlock {
			//kblock is already added to pool, should start with next one
			startHeight := info.PowHeight + 1
			r.logger.Info("Replay pow blocks", "fromHeight", startHeight)
			pool.ReplayFrom(int32(startHeight))
		}
		r.inCommittee = true
		inCommitteeGauge.Set(1)

		// verify in committee status with consent block (1st mblock in epoch)
		// if delegates are obtained from staking
		if !r.config.InitCfgdDelegates && !bestIsKBlock {
			consentBlock, err := r.chain.GetTrunkBlock(kBlockHeight + 1)
			if err != nil {
				fmt.Println("could not get committee info:", err)
				return errors.New("could not get committee info for status check")
			}
			// recover actual committee from consent block
			committeeInfo := consentBlock.CommitteeInfos
			// leaderIndex := committeeInfo.CommitteeInfo[0].CSIndex
			r.UpdateActualCommittee()

			myself := r.curCommittee.Validators[r.curCommitteeIndex]
			myEcdsaPKBytes := crypto.FromECDSAPub(&myself.PubKey)
			inCommitteeVerified := false
			for _, v := range committeeInfo.CommitteeInfo {
				if bytes.Equal(v.PubKey, myEcdsaPKBytes) {
					inCommitteeVerified = true
					break
				}
			}
			if !inCommitteeVerified {
				r.logger.Error("committee info in consent block doesn't contain myself as a member, stop right now")
				return errors.New("committee info in consent block doesnt match myself")
			}
		}

	} else {
		r.inCommittee = false
		inCommitteeGauge.Set(0)
		r.logger.Info("I am NOT in committee!!!", "nonce", nonce)
	}
	return nil
}

// since votes of pacemaker include propser, but committee votes
// do not have leader itself, we seperate the majority func
// Easier adjust the logic of major 2/3, for pacemaker
func MajorityTwoThird(voterNum, committeeSize uint32) bool {
	if committeeSize < 1 {
		fmt.Println("MajorityTwoThird, committee size < 1", committeeSize)
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
	return float64(voterNum) >= twoThirds
}

func (r *Reactor) splitPubKey(comboPub string) (*ecdsa.PublicKey, *bls.PublicKey) {
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
	blsPub, err := r.blsCommon.GetSystem().PubKeyFromBytes(blsPubBytes)
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}

	return pubKey, &blsPub
}

func (r *Reactor) combinePubKey(ecdsaPub *ecdsa.PublicKey, blsPub *bls.PublicKey) string {
	ecdsaPubBytes := crypto.FromECDSAPub(ecdsaPub)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)

	blsPubBytes := r.blsCommon.GetSystem().PubKeyToBytes(*blsPub)
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	return strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
}

func (r *Reactor) GetCombinePubKey() string {
	return r.combinePubKey(&r.myPubKey, &r.blsCommon.PubKey)
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
func (r *Reactor) GetConsensusDelegates() ([]*types.Delegate, int, int) {
	forceDelegates := r.config.InitCfgdDelegates

	// special handle for flag --init-configured-delegates
	var delegates []*types.Delegate
	var err error
	hint := ""
	if forceDelegates {
		delegates = r.config.InitDelegates
		r.sourceDelegates = fromDelegatesFile
		hint = "Loaded delegates from delegates.json"
	} else {
		delegates, err = r.getDelegatesFromStaking()
		hint = "Loaded delegates from staking"
		r.sourceDelegates = fromStaking
		if err != nil || len(delegates) < r.config.MinCommitteeSize {
			delegates = r.config.InitDelegates
			hint = "Loaded delegates from delegates.json as fallback, error loading staking candiates"
			r.sourceDelegates = fromDelegatesFile
		}
	}

	delegateSize, committeeSize := calcCommitteeSize(len(delegates), r.config)
	r.allDelegates = delegates

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

	r.logger.Info(hint, "delegateSize", delegateSize, "committeeSize", committeeSize, "first3", strings.Join(first3Names, ","))
	// PrintDelegates(delegates[:delegateSize])
	return delegates, delegateSize, committeeSize
}

func (r *Reactor) GetDelegateNameByIP(ip net.IP) string {
	for _, d := range r.allDelegates {
		if d.NetAddr.IP.String() == ip.String() {
			return string(d.Name)
		}
	}

	return ""
}

func (r *Reactor) updateCurEpoch(epoch uint64) {
	if epoch > r.curEpoch {
		oldVal := r.curEpoch
		r.curEpoch = epoch
		curEpochGauge.Set(float64(r.curEpoch))
		r.logger.Info("Epoch updated", "from", oldVal, "to", r.curEpoch)
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

func (r *Reactor) OnReceiveMsg(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		r.logger.Error("Unrecognized payload", "err", err)
		return
	}
	mi, err := r.UnmarshalMsg(data)
	if err != nil {
		r.logger.Error("Unmarshal error", "err", err)
		return
	}

	msg, peer := mi.Msg, mi.Peer
	typeName := mi.Msg.GetType()

	signerIndex := msg.GetSignerIndex()
	if int(signerIndex) >= len(r.curActualCommittee) {
		r.logger.Warn("actual committee not initialized, skip relay ...", "msg", msg.GetType())
		return
	}
	signer := r.curActualCommittee[signerIndex]

	if !msg.VerifyMsgSignature(&signer.PubKey) {
		r.logger.Error("invalid signature, dropped ...", "peer", peer, "msg", msg.String(), "signer", signer.Name)
		return
	}
	mi.Signer = signer

	// sanity check for messages
	switch m := msg.(type) {
	case *PMProposalMessage:
		blk := m.DecodeBlock()
		if blk == nil {
			r.logger.Error("Invalid PMProposal: could not decode proposed block")
			return
		}

	case *PMTimeoutMessage:
		qcHigh := m.DecodeQCHigh()
		if qcHigh == nil {
			r.logger.Error("Invalid QCHigh: could not decode qcHigh")
		}
	}

	fromMyself := peer.IP == r.GetMyNetAddr().IP.String()
	suffix := ""
	if fromMyself {
		suffix = " (myself)"
	}

	if msg.GetEpoch() < r.curEpoch {
		r.logger.Info(fmt.Sprintf("recv outdated %s", msg.String()), "peer", peer.String()+suffix)
		return
	}

	if r.pacemaker == nil {
		r.logger.Warn("pacemaker is not initialized, dropped message")
		return
	}
	err = r.inQueue.Add(mi)
	if err != nil {
		return
	}

	// relay the message if these two conditions are met:
	// 1. the original message is not sent by myself
	// 2. it's a proposal message
	if !fromMyself && typeName == "PMProposal" {
		r.Relay(mi.Msg, data)
	}
}
