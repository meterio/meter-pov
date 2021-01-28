// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	sha256 "crypto/sha256"
	"encoding/base64"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	cli "gopkg.in/urfave/cli.v1"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/comm"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/genesis"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/inconshreveable/log15"
)

const (
	//maxMsgSize = 1048576 // 1MB;
	// set as 1184 * 1024
	maxMsgSize  = 1300000 // gasLimit 20000000 generate, 1024+1024 (1048576) + sizeof(QC) + sizeof(committee)...
	MsgHashSize = 8

	//normally when a block is committed, wait for a while to let whole network to sync and move to next round
	WHOLE_NETWORK_BLOCK_SYNC_TIME = 5 * time.Second

	// Sign Announce Mesage
	// "Announce Committee Message: Leader <pubkey 64(hexdump 32x2) bytes> EpochID <8 (4x2)bytes> Height <16 (8x2) bytes> Round <8(4x2)bytes>
	// ANNOUNCE_SIGN_MSG_SIZE = int(110)

	// Sign Propopal Message
	// "Proposal Block Message: Proposer <pubkey 64(32x3)> BlockType <2 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
	// PROPOSAL_SIGN_MSG_SIZE = int(100)

	// Sign Notary Announce Message
	// "Announce Notarization Message: Leader <pubkey 64(32x3)> EpochID <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
	// NOTARY_ANNOUNCE_SIGN_MSG_SIZE = int(120)

	// Sign Notary Block Message
	// "Block Notarization Message: Proposer <pubkey 64(32x3)> BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
	// NOTARY_BLOCK_SIGN_MSG_SIZE = int(130)

	CHAN_DEFAULT_BUF_SIZE = 100

	DEFAULT_EPOCHS_PERDAY = 24
)

const (
	fromDelegatesFile = iota
	fromStaking
)

var (
	ConsensusGlobInst *ConsensusReactor
)

var (
	ErrUnrecognizedPayload = errors.New("unrecognized payload")
	ErrMagicMismatch       = errors.New("magic mismatch")
	ErrMalformattedMsg     = errors.New("Malformatted msg")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrInvalidMsgType      = errors.New("invalid msg type")
)

type ConsensusConfig struct {
	ForceLastKFrame    bool
	SkipSignatureCheck bool
	InitCfgdDelegates  bool
	EpochMBlockCount   uint32
	MinCommitteeSize   int
	MaxCommitteeSize   int
	MaxDelegateSize    int
	InitDelegates      []*types.Delegate
}

//-----------------------------------------------------------------------------
// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	chain        *chain.Chain
	stateCreator *state.Creator

	config   ConsensusConfig
	SyncDone bool

	// copy of master/node
	myPubKey      ecdsa.PublicKey  // this is my public identification !!
	myPrivKey     ecdsa.PrivateKey // copy of private key
	myBeneficiary meter.Address

	// still references above consensuStae, reactor if this node is
	// involved the consensus
	csMode             byte // delegates, committee, other
	delegateSize       int  // global constant, current available delegate size.
	committeeSize      uint32
	curDelegates       *types.DelegateSet      // current delegates list
	curCommittee       *types.ValidatorSet     // This is top 400 of delegates by given nonce
	curActualCommittee []types.CommitteeMember // Real committee, should be subset of curCommittee if someone is offline.
	curCommitteeIndex  uint32
	logger             log15.Logger

	csRoleInitialized uint
	csCommon          *types.ConsensusCommon //this must be allocated as validator
	csLeader          *ConsensusLeader
	//	csProposer        *ConsensusProposer
	csValidator *ConsensusValidator
	csPacemaker *Pacemaker

	// store key states here
	lastKBlockHeight uint32
	curNonce         uint64
	curEpoch         uint64
	curHeight        uint32 // come from parentBlockID first 4 bytes uint32
	mtx              sync.RWMutex

	// TODO: remove this, not used anymore
	kBlockData *block.KBlockData
	// consensus state for new consensus, similar to old conS

	// state changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan consensusMsgInfo
	internalMsgQueue chan consensusMsgInfo
	schedulerQueue   chan func()

	// kBlock data
	KBlockDataQueue    chan block.KBlockData // from POW simulation
	RcvKBlockInfoQueue chan RecvKBlockInfo   // this channel for kblock notify from node module.

	newCommittee     *NewCommittee                     //New committee for myself
	rcvdNewCommittee map[NewCommitteeKey]*NewCommittee // store received new committee info

	msgCache *MsgCache

	magic           [4]byte
	inCommittee     bool
	allDelegates    []*types.Delegate
	sourceDelegates int
}

// Glob Instance
func GetConsensusGlobInst() *ConsensusReactor {
	return ConsensusGlobInst
}

func SetConsensusGlobInst(inst *ConsensusReactor) {
	ConsensusGlobInst = inst
}

// NewConsensusReactor returns a new ConsensusReactor with the given
// consensusState.
func NewConsensusReactor(ctx *cli.Context, chain *chain.Chain, state *state.Creator, privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey, magic [4]byte, blsCommon *BlsCommon, initDelegates []*types.Delegate) *ConsensusReactor {
	conR := &ConsensusReactor{
		chain:        chain,
		stateCreator: state,
		logger:       log15.New("pkg", "reactor"),
		SyncDone:     false,
		magic:        magic,
		msgCache:     NewMsgCache(1024),
		inCommittee:  false,
	}

	if ctx != nil {
		conR.config = ConsensusConfig{
			ForceLastKFrame:    ctx.Bool("force-last-kframe"),
			SkipSignatureCheck: ctx.Bool("skip-signature-check"),
			InitCfgdDelegates:  ctx.Bool("init-configured-delegates"),
			EpochMBlockCount:   uint32(ctx.Uint("epoch-mblock-count")),
			MinCommitteeSize:   ctx.Int("committee-min-size"),
			MaxCommitteeSize:   ctx.Int("committee-max-size"),
			MaxDelegateSize:    ctx.Int("delegate-max-size"),
			InitDelegates:      initDelegates,
		}
	}

	//initialize message channel
	conR.peerMsgQueue = make(chan consensusMsgInfo, CHAN_DEFAULT_BUF_SIZE)
	conR.internalMsgQueue = make(chan consensusMsgInfo, CHAN_DEFAULT_BUF_SIZE)
	conR.schedulerQueue = make(chan func(), CHAN_DEFAULT_BUF_SIZE)
	conR.KBlockDataQueue = make(chan block.KBlockData, CHAN_DEFAULT_BUF_SIZE)

	// add the hardcoded genesis nonce in the case every node in block 0
	conR.RcvKBlockInfoQueue = make(chan RecvKBlockInfo, CHAN_DEFAULT_BUF_SIZE)

	//initialize height/round
	conR.lastKBlockHeight = chain.BestBlock().Header().LastKBlockHeight()
	conR.curHeight = chain.BestBlock().Header().Number()

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

	prometheus.MustRegister(pmRoundGauge)
	prometheus.MustRegister(curEpochGauge)
	prometheus.MustRegister(lastKBlockHeightGauge)
	prometheus.MustRegister(blocksCommitedCounter)
	prometheus.MustRegister(inCommitteeGauge)
	prometheus.MustRegister(pmRoleGauge)

	lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))

	//initialize Delegates

	conR.UpdateCurDelegates()

	conR.rcvdNewCommittee = make(map[NewCommitteeKey]*NewCommittee, 10)

	conR.myPrivKey = *privKey
	conR.myPubKey = *pubKey

	SetConsensusGlobInst(conR)
	return conR
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (conR *ConsensusReactor) OnStart() error {
	// Start new consensus
	conR.NewConsensusStart()

	// force to receive nonce
	//conR.ConsensusHandleReceivedNonce(0, 1001)

	conR.logger.Info("Consensus started ... ", "curHeight", conR.curHeight)
	return nil
}

// OnStop implements BaseService by unsubscribing from events and stopping
// state.
func (conR *ConsensusReactor) OnStop() {
	// New consensus
	conR.NewConsensusStop()
}

func (conR *ConsensusReactor) GetLastKBlockHeight() uint32 {
	return conR.lastKBlockHeight
}

// SwitchToConsensus switches from fast_sync mode to consensus mode.
// It resets the state, turns off fast_sync, and starts the consensus state-machine
func (conR *ConsensusReactor) SwitchToConsensus() {
	//conR.Logger.Info("SwitchToConsensus")
	conR.logger.Info("Synchnization is done. SwitchToConsensus ...")

	var nonce uint64
	best := conR.chain.BestBlock()
	bestKBlock, err := conR.chain.BestKBlock()
	if err != nil {
		panic("could not get best KBlock")
	}
	if bestKBlock.Header().Number() == 0 {
		nonce = genesis.GenesisNonce
	} else {
		nonce = bestKBlock.KBlockData.Nonce
	}
	replay := (best.Header().Number() != bestKBlock.Header().Number())

	// --force-last-kframe
	if !conR.config.ForceLastKFrame {
		conR.JoinEstablishedCommittee(bestKBlock, replay)
	} else {
		conR.ConsensusHandleReceivedNonce(bestKBlock.Header().Number(), nonce, best.QC.EpochID, replay)
	}
}

// String returns a string representation of the ConsensusReactor.
// NOTE: For now, it is just a hard-coded string to avoid accessing unprotected shared variables.
// TODO: improve!
func (conR *ConsensusReactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor" // conR.StringIndented("")
}

//-----------------------------------------------------------------------------
//----new consensus-------------------------------------------------------------
//-----------------------------------------------------------------------------
//define the node mode
// 1. CONSENSUS_MODE_OTHER
// 2. CONSENSUS_MODE_OBSERVER
// 3. CONSENSUS_MODE_DELEGATE
// 4. CONSENSUS_MODE_COMMITTEE

const (
	CONSENSUS_MODE_OTHER     = byte(0x01)
	CONSENSUS_MODE_OBSERVER  = byte(0x02)
	CONSENSUS_MODE_DELEGATE  = byte(0x03)
	CONSENSUS_MODE_COMMITTEE = byte(0x04)

	// Flags of Roles
	CONSENSUS_COMMIT_ROLE_NONE      = uint(0x0)
	CONSENSUS_COMMIT_ROLE_LEADER    = uint(0x01)
	CONSENSUS_COMMIT_ROLE_PROPOSER  = uint(0x02)
	CONSENSUS_COMMIT_ROLE_VALIDATOR = uint(0x04)
)

type consensusMsgInfo struct {
	//Msg    ConsensusMessage
	Msg       ConsensusMessage
	Peer      *ConsensusPeer
	RawData   []byte
	Signature []byte

	cache struct {
		msgHash    [32]byte
		msgHashHex string
	}
}

func newConsensusMsgInfo(msg ConsensusMessage, peer *ConsensusPeer, rawData []byte) *consensusMsgInfo {
	return &consensusMsgInfo{
		Msg:       msg,
		Peer:      peer,
		RawData:   rawData,
		Signature: msg.Header().Signature,
		cache: struct {
			msgHash    [32]byte
			msgHashHex string
		}{msgHashHex: ""},
	}
}

func (mi *consensusMsgInfo) MsgHashHex() string {
	if mi.cache.msgHashHex == "" {
		msgHash := sha256.Sum256(mi.RawData)
		msgHashHex := hex.EncodeToString(msgHash[:])[:8]
		mi.cache.msgHash = msgHash
		mi.cache.msgHashHex = msgHashHex
	}
	return mi.cache.msgHashHex

}

func (conR *ConsensusReactor) UpdateHeight(height uint32) bool {
	conR.logger.Info(fmt.Sprintf("Update conR.curHeight from %d to %d", conR.curHeight, height))
	conR.curHeight = height
	return true
}

// update the LastKBlockHeight
func (conR *ConsensusReactor) UpdateLastKBlockHeight(height uint32) bool {
	conR.lastKBlockHeight = height
	lastKBlockHeightGauge.Set(float64(height))
	return true
}

// Refresh the current Height from the best block
// normally call this routine after block chain changed
func (conR *ConsensusReactor) RefreshCurHeight() error {
	prev := conR.curHeight

	best := conR.chain.BestBlock()
	conR.curHeight = best.Header().Number()
	conR.lastKBlockHeight = best.Header().LastKBlockHeight()
	conR.updateCurEpoch(best.GetBlockEpoch())

	lastKBlockHeightGauge.Set(float64(conR.lastKBlockHeight))
	conR.logger.Info("Refresh curHeight", "from", prev, "to", conR.curHeight, "lastKBlock", conR.lastKBlockHeight, "epoch", conR.curEpoch)
	return nil
}

// after announce/commit, Leader got the actual committee, which is the subset of curCommittee if some committee member offline.
// indexs and pubKeys are not sorted slice, AcutalCommittee must be sorted.
// Only Leader can call this method. indexes do not include the leader itself.
func (conR *ConsensusReactor) UpdateActualCommittee(leaderIndex uint32, config ConsensusConfig) bool {
	size := len(conR.curCommittee.Validators)
	//validators := conR.curCommittee.Validators
	validators := make([]*types.Validator, 0)
	if config.InitCfgdDelegates {
		validators = append(validators, conR.curCommittee.Validators...)
	} else {
		// put leader the first in committee
		// only if delegates are obtained from staking
		validators = append(validators, conR.curCommittee.Validators[leaderIndex:]...)
		validators = append(validators, conR.curCommittee.Validators[:leaderIndex]...)
	}
	for i, v := range validators {
		cm := types.CommitteeMember{
			Name:     v.Name,
			PubKey:   v.PubKey,
			NetAddr:  v.NetAddr,
			CSPubKey: v.BlsPubKey,
			CSIndex:  (i + int(leaderIndex)) % size,
		}
		conR.curActualCommittee = append(conR.curActualCommittee, cm)
	}

	// I am Leader, first one should be myself.
	// if bytes.Equal(crypto.FromECDSAPub(&conR.curActualCommittee[0].PubKey), crypto.FromECDSAPub(&conR.myPubKey)) == false {
	// conR.logger.Error("I am leader and not in first place of curActualCommittee, must correct !!!")
	// return false
	// }

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

//create validatorSet by a given nonce. return by my self role
func (conR *ConsensusReactor) NewValidatorSetByNonce(nonce uint64) (uint, bool) {
	committee, role, index, inCommittee := conR.CalcCommitteeByNonce(nonce)
	// fmt.Println("CALCULATED COMMITEE", "role=", role, "index=", index)
	// fmt.Println(committee)
	conR.curCommittee = committee
	if inCommittee == true {
		conR.csMode = CONSENSUS_MODE_COMMITTEE
		conR.curCommitteeIndex = uint32(index)
		myAddr := conR.curCommittee.Validators[index].NetAddr
		myName := conR.curCommittee.Validators[index].Name
		conR.logger.Info("New committee calculated", "index", index, "role", role, "myName", myName, "myIP", myAddr.IP.String())
	} else {
		conR.csMode = CONSENSUS_MODE_DELEGATE
		conR.curCommitteeIndex = 0
		// FIXME: find a better way
		conR.logger.Info("New committee calculated")
	}
	fmt.Println(committee)

	return role, inCommittee
}

//This is similar routine of NewValidatorSetByNonce.
//it is used for temp calculate committee set by a given nonce in the fly.
// also return the committee
func (conR *ConsensusReactor) CalcCommitteeByNonce(nonce uint64) (*types.ValidatorSet, uint, int, bool) {
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

		return Committee, CONSENSUS_COMMIT_ROLE_NONE, 0, false
	}

	if bytes.Equal(crypto.FromECDSAPub(&vals[0].PubKey), crypto.FromECDSAPub(&conR.myPubKey)) == true {
		return Committee, CONSENSUS_COMMIT_ROLE_LEADER, 0, true
	}

	for i, val := range vals {
		if bytes.Equal(crypto.FromECDSAPub(&val.PubKey), crypto.FromECDSAPub(&conR.myPubKey)) == true {
			return Committee, CONSENSUS_COMMIT_ROLE_VALIDATOR, i, true
		}
	}

	return Committee, CONSENSUS_COMMIT_ROLE_NONE, 0, false
}

func (conR *ConsensusReactor) GetCommitteeMemberIndex(pubKey ecdsa.PublicKey) int {
	for i, v := range conR.curCommittee.Validators {
		if bytes.Equal(crypto.FromECDSAPub(&v.PubKey), crypto.FromECDSAPub(&pubKey)) == true {
			return i
		}
	}
	conR.logger.Error("I'm not in committee, please check public key settings", "pubKey", pubKey)
	return -1
}

func (conR *ConsensusReactor) GetActualCommitteeMemberIndex(pubKey *ecdsa.PublicKey) int {
	for i, member := range conR.curActualCommittee {
		if bytes.Equal(crypto.FromECDSAPub(&member.PubKey), crypto.FromECDSAPub(pubKey)) == true {
			return i
		}
	}
	conR.logger.Error("public key not found in actual committee", "pubKey", pubKey)
	return -1
}

// input is serialized ecdsa.PublicKey
func (conR *ConsensusReactor) GetCommitteeMember(pubKey []byte) *types.CommitteeMember {
	for _, v := range conR.curActualCommittee {
		if bytes.Equal(crypto.FromECDSAPub(&v.PubKey), pubKey) == true {
			return &v
		}
	}
	conR.logger.Error("not found", "pubKey", pubKey)
	return nil
}

// Handle received Message
func (conR *ConsensusReactor) handleMsg(mi consensusMsgInfo) {
	conR.mtx.Lock()
	defer conR.mtx.Unlock()

	msg, peer := mi.Msg, mi.Peer

	typeName := getConcreteName(msg)
	// msgHashHex := hex.EncodeToString(mi.MsgHash[:])[:MsgHashSize]
	peerIP := peer.netAddr.IP.String()
	// conR.logger.Info(fmt.Sprintf("start to handle msg: %v", typeName),
	// 	"peer", peer.name,
	// 	"ip", peerIP, "msgHash", msgHashHex)

	var success bool
	switch msg := msg.(type) {

	// New consensus Messages
	case *AnnounceCommitteeMessage:
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			conR.logger.Warn("not in validator role, enter validator first ...")
			// if find out we are not in committee, then exit validator
			conR.enterConsensusValidator()
		}

		success = conR.csValidator.ProcessAnnounceCommittee(msg, peer)
		// For ProcessAnnounceCommittee, it is not validator if return is false
		if success == false {
			conR.logger.Error("process announce failed")
			conR.exitConsensusValidator()
		}

	case *CommitCommitteeMessage:
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_LEADER) == 0 ||
			(conR.csLeader == nil) {
			conR.logger.Warn("not in leader role, ignore CommitCommitteeMessage")
			break
		}

		success = conR.csLeader.ProcessCommitMsg(msg, peer)
		if success == false {
			conR.logger.Error("process CommitCommitteeMessage failed")
		}

	case *NotaryAnnounceMessage:
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			conR.logger.Warn("not in validator role, ignore NotaryAnnounceMessage")
			break
		}

		success = conR.csValidator.ProcessNotaryAnnounceMessage(msg, peer)
		if success == false {
			conR.logger.Error("process NotaryAnnounceMessage failed")
		}

	case *NewCommitteeMessage:
		success = conR.ProcessNewCommitteeMessage(msg, peer)
		if success == false {
			conR.logger.Error("process NewcommitteeMessage failed")
		}
	default:
		conR.logger.Error("Unknown msg type", "value", reflect.TypeOf(msg))
	}

	// relay the message
	fromMyself := peerIP == conR.GetMyNetAddr().IP.String()
	if conR.inCommittee && fromMyself == false && success == true {
		// relay only if these three conditions meet:
		// 1. I'm in committee
		// 2. the message is not from myself
		// 3. message is proved to be valid
		if typeName == "AnnounceCommittee" {
			conR.relayMsg(mi, int(conR.newCommittee.Round))
		} else if typeName == "NotaryAnnounce" {
			conR.relayMsg(mi, int(msg.Header().Round))
		}
	}
}

func (conR *ConsensusReactor) GetRelayPeers(round int) ([]*ConsensusPeer, error) {
	peers := make([]*ConsensusPeer, 0)
	size := len(conR.curCommittee.Validators)
	myIndex := int(conR.curCommitteeIndex)
	if size == 0 {
		return peers, errors.New("current actual committee is empty")
	}
	rr := round % size
	if myIndex >= rr {
		myIndex = myIndex - rr
	} else {
		myIndex = myIndex + size - rr
	}

	indexes := GetRelayPeers(myIndex, size-1)
	for _, i := range indexes {
		index := i + rr
		if index >= size {
			index = index % size
		}
		member := conR.curCommittee.Validators[index]
		name := conR.GetDelegateNameByIP(member.NetAddr.IP)
		peers = append(peers, newConsensusPeer(name, member.NetAddr.IP, member.NetAddr.Port, conR.magic))
	}
	return peers, nil
}

func (conR *ConsensusReactor) relayMsg(mi consensusMsgInfo, round int) {
	peers, _ := conR.GetRelayPeers(round)
	typeName := getConcreteName(mi.Msg)
	conR.logger.Info("Now, relay committee msg", "type", typeName, "round", round)
	for _, peer := range peers {
		msgSummary := (mi.Msg).String()
		go peer.sendCommitteeMsg(mi.RawData, msgSummary, true)
	}
	// conR.asyncSendCommitteeMsg(msg, true, peers...)
}

// receiveRoutine handles messages which may cause state transitions.
func (conR *ConsensusReactor) receiveRoutine() {
	/******
	onExit := func(conR *ConsensusReactor) {
		// NOTE: the internalMsgQueue may have signed messages from our
		// fmt.Println("Exiting receiveRoutine ... ", "height ", conR.curHeight, "round ", conR.curRound)
		return
	}
	*******/
	//wait for synchronization is done
	communicator := comm.GetGlobCommInst()
	if communicator == nil {
		conR.logger.Error("get communicator instance failed ...")
		return
	}
	select {
	case <-communicator.Synced():
		conR.SwitchToConsensus()
	}
	conR.logger.Info("Sync is done, start to accept consensus message")
	conR.SyncDone = true

	for {
		var mi consensusMsgInfo
		select {
		case mi = <-conR.peerMsgQueue:
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			// conR.logger.Debug("Received message from peerMsgQueue")
			conR.handleMsg(mi)
		case mi = <-conR.internalMsgQueue:
			// handles proposals, block parts, votes
			conR.logger.Debug("Received message from InternalMsgQueue")
			conR.handleMsg(mi)
		case ti := <-conR.schedulerQueue:
			conR.HandleSchedule(ti)

		case ki := <-conR.RcvKBlockInfoQueue:
			conR.HandleRecvKBlockInfo(ki)

		case kd := <-conR.KBlockDataQueue:
			conR.HandleKBlockData(kd)

			/*******
			case pi := <-conR.packerInfoQueue:
				conR.HandlePackerInfo(pi)
			case <-conR.Quit():
				onExit(conR)
			************/
		}
	}
}

func (conR *ConsensusReactor) MarshalMsg(msg *ConsensusMessage) ([]byte, error) {
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		conR.logger.Error("Msg exceeds max size", "rawMsg", len(rawMsg), "maxMsgSize", maxMsgSize)
		return make([]byte, 0), errors.New("Msg exceeds max size")
	}

	magicHex := hex.EncodeToString(conR.magic[:])
	myNetAddr := conR.GetMyNetAddr()
	payload := map[string]interface{}{
		"message":   hex.EncodeToString(rawMsg),
		"peer_ip":   myNetAddr.IP.String(),
		"peer_port": strconv.Itoa(int(myNetAddr.Port)),
		"magic":     magicHex,
	}

	return json.Marshal(payload)
}

func (conR *ConsensusReactor) UnmarshalMsg(data []byte) (*consensusMsgInfo, error) {
	var params map[string]string
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&params)
	if err != nil {
		fmt.Println(err)
		return nil, ErrUnrecognizedPayload
	}
	if strings.Compare(params["magic"], hex.EncodeToString(conR.magic[:])) != 0 {
		return nil, ErrMagicMismatch
	}
	peerIP := net.ParseIP(params["peer_ip"])
	peerPort, err := strconv.ParseUint(params["peer_port"], 10, 16)
	if err != nil {
		fmt.Println("Unrecognized Payload: ", err)
		return nil, ErrUnrecognizedPayload
	}
	peerName := conR.GetDelegateNameByIP(peerIP)
	peer := newConsensusPeer(peerName, peerIP, uint16(peerPort), conR.magic)
	rawMsg, err := hex.DecodeString(params["message"])
	if err != nil {
		fmt.Println("could not decode string: ", params["message"])
		return nil, ErrMalformattedMsg
	}
	msg, err := decodeMsg(rawMsg)
	if err != nil {
		fmt.Println("Malformatted Msg: ", msg)
		return nil, ErrMalformattedMsg
		// conR.logger.Error("Malformated message, error decoding", "peer", peerName, "ip", peerIP, "msg", msg, "err", err)
	}

	return newConsensusMsgInfo(msg, peer, data), nil
}

func (conR *ConsensusReactor) ReceivePacemakerMsg(w http.ResponseWriter, r *http.Request) {
	if conR.csPacemaker != nil {
		conR.csPacemaker.receivePacemakerMsg(w, r)
	} else {
		conR.logger.Warn("pacemaker is not initialized, dropped message")
	}
}

func (conR *ConsensusReactor) ReceiveCommitteeMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
		return
	}
	mi, err := conR.UnmarshalMsg(data)
	if err != nil {
		fmt.Println(err)
		return
	}

	// cache the message to avoid duplicate handling
	msg, sig, peer := mi.Msg, mi.Signature, mi.Peer
	peerName := peer.name
	peerIP := peer.netAddr.IP.String()
	typeName := getConcreteName(msg)
	existed := conR.msgCache.Add(sig)
	if existed {
		conR.logger.Debug("duplicate "+typeName+", dropped ...", "epoch", msg.EpochID(), "peer", peerName, "ip", peerIP)
		return
	}

	if VerifyMsgType(msg) == false {
		conR.logger.Error("invalid msg type, dropped ...", "peer", peerName, "ip", peerIP, "msg", msg.String())
		return
	}

	if VerifySignature(msg) == false {
		conR.logger.Error("invalid signature, dropped ...", "peer", peerName, "ip", peerIP, "msg", msg.String())
		return
	}

	conR.logger.Info(fmt.Sprintf("Recv %s", msg.String()), "peer", peerName, "ip", peerIP, "msgHash", mi.MsgHashHex())

	conR.peerMsgQueue <- *mi
	// respondWithJson(w, http.StatusOK, map[string]string{"result": "success"})
}

/*
func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
*/

//Entry point of new consensus
func (conR *ConsensusReactor) NewConsensusStart() int {
	conR.logger.Debug("Starting New Consensus ...")

	/***** Common init is based on roles
	 ***** Leader generate announce message.
	 ***** Validators receives announce and do committee init
	 */

	// Uncomment following to enable peer messages between nodes
	// go conR.receiveConsensusMsgRoutine()

	// Start receive routine
	go conR.receiveRoutine() //only handles from channel
	return 0
}

// called by reactor stop
func (conR *ConsensusReactor) NewConsensusStop() int {
	conR.logger.Warn("Stop New Consensus ...")

	// Deinitialize consensus common
	conR.csCommon.Destroy()
	return 0
}

// -------
// Enter validator
func (conR *ConsensusReactor) enterConsensusValidator() int {
	conR.logger.Debug("Enter consensus validator")

	conR.csValidator = NewConsensusValidator(conR)
	conR.csRoleInitialized |= CONSENSUS_COMMIT_ROLE_VALIDATOR
	conR.csValidator.replay = conR.newCommittee.Replay //replay flag carried by newcommittee

	return 0
}

func (conR *ConsensusReactor) exitConsensusValidator() int {

	conR.csValidator = nil
	conR.csRoleInitialized &= ^CONSENSUS_COMMIT_ROLE_VALIDATOR
	conR.msgCache.CleanAll()
	conR.logger.Debug("Exit consensus validator")
	return 0
}

// Enter leader
func (conR *ConsensusReactor) enterConsensusLeader(epochID uint64) {
	conR.logger.Debug("Enter consensus leader", "epochID", epochID)
	if epochID <= conR.curEpoch && conR.csLeader != nil {
		conR.logger.Warn("not update leader", "curEpochID", conR.curEpoch, "epochID", epochID)
		return
	}

	conR.csLeader = NewCommitteeLeader(conR)
	conR.csRoleInitialized |= CONSENSUS_COMMIT_ROLE_LEADER

	conR.updateCurEpoch(epochID)
	conR.csLeader.EpochID = epochID
	return
}

func (conR *ConsensusReactor) exitConsensusLeader(epochID uint64) {
	conR.logger.Warn("Exit consensus leader", "epochID", epochID)
	if conR.curEpoch != epochID {
		conR.logger.Warn("exitConsensusLeader: epochID mismatch, do not update leader", "curEpochID", conR.curEpoch, "epochID", epochID)
		return
	}
	conR.csLeader = nil
	conR.csRoleInitialized &= ^CONSENSUS_COMMIT_ROLE_LEADER

	conR.msgCache.CleanAll()
	return
}

// Cleanup all roles before the comittee relay
func (conR *ConsensusReactor) exitCurCommittee() {

	conR.exitConsensusLeader(conR.curEpoch)
	conR.exitConsensusValidator()

	// clean up current parameters
	if conR.curCommittee != nil {
		conR.curCommittee.Validators = make([]*types.Validator, 0)
	}
	conR.curActualCommittee = make([]types.CommitteeMember, 0)
	conR.curCommitteeIndex = 0
	conR.kBlockData = nil

	conR.curNonce = 0

}

func (conR *ConsensusReactor) asyncSendCommitteeMsg(msg *ConsensusMessage, relay bool, peers ...*ConsensusPeer) bool {
	data, err := conR.MarshalMsg(msg)
	if err != nil {
		fmt.Println("Could not marshal message")
		return false
	}
	msgSummary := (*msg).String()

	for _, peer := range peers {
		go peer.sendCommitteeMsg(data, msgSummary, relay)
	}

	//wg.Wait()
	return true
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

func (conR *ConsensusReactor) GetMyPeers() ([]*ConsensusPeer, error) {
	peers := make([]*ConsensusPeer, 0)
	myNetAddr := conR.GetMyNetAddr()
	for _, member := range conR.curActualCommittee {
		if member.NetAddr.IP.String() != myNetAddr.IP.String() {
			peers = append(peers, newConsensusPeer(member.Name, member.NetAddr.IP, member.NetAddr.Port, conR.magic))
		}
	}
	return peers, nil
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

type ApiCommitteeMember struct {
	Name        string
	Address     meter.Address
	PubKey      string
	VotingPower int64
	NetAddr     string
	CsPubKey    string
	CsIndex     int
	InCommittee bool
}

func (conR *ConsensusReactor) GetLatestCommitteeList() ([]*ApiCommitteeMember, error) {
	var committeeMembers []*ApiCommitteeMember
	inCommittee := make([]bool, len(conR.curCommittee.Validators))
	for i := range inCommittee {
		inCommittee[i] = false
	}

	for _, cm := range conR.curActualCommittee {
		v := conR.curCommittee.Validators[cm.CSIndex]
		apiCm := &ApiCommitteeMember{
			Name:        v.Name,
			Address:     v.Address,
			PubKey:      b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&cm.PubKey)),
			VotingPower: v.VotingPower,
			NetAddr:     cm.NetAddr.String(),
			CsPubKey:    hex.EncodeToString(conR.csCommon.GetSystem().PubKeyToBytes(cm.CSPubKey)),
			CsIndex:     cm.CSIndex,
			InCommittee: true,
		}
		// fmt.Println(fmt.Sprintf("set %d to true, with index = %d ", i, cm.CSIndex))
		committeeMembers = append(committeeMembers, apiCm)
		inCommittee[cm.CSIndex] = true
	}
	for i, val := range inCommittee {
		if val == false {
			v := conR.curCommittee.Validators[i]
			apiCm := &ApiCommitteeMember{
				Name:        v.Name,
				Address:     v.Address,
				PubKey:      b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(&v.PubKey)),
				CsIndex:     i,
				InCommittee: false,
			}
			committeeMembers = append(committeeMembers, apiCm)
		}
	}
	return committeeMembers, nil
}

//============================================
func (conR *ConsensusReactor) SignConsensusMsg(msgHash []byte) (sig []byte, err error) {
	sig, err = crypto.Sign(msgHash, &conR.myPrivKey)
	if err != nil {
		return []byte{}, err
	}

	return sig, nil
}

//----------------------------------------------------------------------------
// Sign New Committee
// "New Committee Message: Leader <pubkey 64(hexdump 32x2) bytes> EpochID <16 (8x2)bytes> Height <16 (8x2) bytes>
func (conR *ConsensusReactor) BuildNewCommitteeSignMsg(pubKey ecdsa.PublicKey, epochID uint64, height uint64) string {
	c := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(c, epochID)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	return fmt.Sprintf("%s %s %s %s %s %s", "New Committee Message: Leader", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"EpochID", hex.EncodeToString(c), "Height", hex.EncodeToString(h))
}

//----------------------------------------------------------------------------
// Sign Announce Committee
// "Announce Committee Message: Leader <pubkey 64(hexdump 32x2) bytes> EpochID <16 (8x2)bytes> Height <16 (8x2) bytes> Round <8(4x2)bytes>
func (conR *ConsensusReactor) BuildAnnounceSignMsg(pubKey ecdsa.PublicKey, epochID uint64, height uint64, round uint32) string {
	c := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(c, epochID)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	r := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(r, round)
	return fmt.Sprintf("%s %s %s %s %s %s %s %s", "Announce Committee Message: Leader", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"EpochID", hex.EncodeToString(c), "Height", hex.EncodeToString(h),
		"Round", hex.EncodeToString(r))
}

// Sign Propopal Message
// "Proposal Block Message: BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func (conR *ConsensusReactor) BuildProposalBlockSignMsg(blockType uint32, height uint64, id, txsRoot, stateRoot *meter.Bytes32) string {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, blockType)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	return fmt.Sprintf("%s %s %s %s %s %s %s %s %s %s",
		"BlockType", hex.EncodeToString(c),
		"Height", hex.EncodeToString(h),
		"BlockID", id.String(),
		"TxRoot", txsRoot.String(),
		"StateRoot", stateRoot.String())
}

// Sign Notary Announce Message
// "Announce Notarization Message: Leader <pubkey 64(32x2)> EpochID <16 (8x2)bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func (conR *ConsensusReactor) BuildNotaryAnnounceSignMsg(pubKey ecdsa.PublicKey, epochID uint64, height uint64, round uint32) string {
	c := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(c, epochID)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	r := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(r, round)
	return fmt.Sprintf("%s %s %s %s %s %s %s %s", "Announce Notarization Message: Leader", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"EpochID", hex.EncodeToString(c), "Height", hex.EncodeToString(h),
		"Round", hex.EncodeToString(r))
}

//======end of New consensus =========================================
//-----------------------------------------------------------------------------
// New consensus timed schedule util
//type Scheduler func(conR *ConsensusReactor) bool
func (conR *ConsensusReactor) ScheduleLeader(epochID uint64, height uint32, round uint32, ev *NCEvidence, d time.Duration) bool {
	time.AfterFunc(d, func() {
		conR.schedulerQueue <- func() { HandleScheduleLeader(conR, epochID, height, round, ev) }
	})
	return true
}

func (conR *ConsensusReactor) ScheduleReplayLeader(epochID uint64, height uint32, round uint32, ev *NCEvidence, d time.Duration) bool {
	time.AfterFunc(d, func() {
		conR.schedulerQueue <- func() { HandleScheduleReplayLeader(conR, epochID, height, round, ev) }
	})
	return true
}

// -------------------------------
func HandleScheduleReplayLeader(conR *ConsensusReactor, epochID uint64, height uint32, round uint32, ev *NCEvidence) bool {
	conR.exitConsensusLeader(conR.curEpoch)

	conR.logger.Debug("Enter consensus replay leader", "curEpochID", conR.curEpoch, "epochID", epochID)

	// init consensus common as leader
	// need to deinit to avoid the memory leak
	best := conR.chain.BestBlock()
	curHeight := best.Header().Number()
	if height != curHeight {
		conR.logger.Warn("height is not the same with curHeight")
	}

	lastKBlockHeight := best.Header().LastKBlockHeight()
	lastKBlockHeightGauge.Set(float64(lastKBlockHeight))

	conR.csLeader = NewCommitteeLeader(conR)
	conR.csRoleInitialized |= CONSENSUS_COMMIT_ROLE_LEADER
	conR.csLeader.replay = true
	conR.updateCurEpoch(epochID)
	conR.csLeader.EpochID = epochID
	conR.csLeader.voterMsgHash = ev.voterMsgHash
	conR.csLeader.voterBitArray = ev.voterBitArray
	conR.csLeader.voterAggSig = ev.voterAggSig

	conR.csLeader.GenerateAnnounceMsg(curHeight, round)
	return true
}

func HandleScheduleLeader(conR *ConsensusReactor, epochID uint64, height uint32, round uint32, ev *NCEvidence) bool {
	curHeight := conR.chain.BestBlock().Header().Number()
	if curHeight != height {
		conR.logger.Error("ScheduleLeader: best height is different with kblock height", "curHeight", curHeight, "kblock height", height)
		if curHeight > height {
			// mine is ahead of kblock, stop
			return false
		}
		com := comm.GetGlobCommInst()
		if com == nil {
			conR.logger.Error("get global comm inst failed")
			return false
		}
		com.TriggerSync()
		conR.logger.Warn("Peer sync triggered")

		conR.ScheduleLeader(epochID, height, round, ev, 1*time.Second)
		return false
	}

	conR.enterConsensusLeader(epochID)

	conR.csLeader.voterBitArray = ev.voterBitArray
	conR.csLeader.voterMsgHash = ev.voterMsgHash
	conR.csLeader.voterAggSig = ev.voterAggSig
	conR.csLeader.GenerateAnnounceMsg(height, round)
	return true
}

// Handle Schedules from conR.scheduleQueue
func (conR *ConsensusReactor) HandleSchedule(fn func()) bool {
	/***
	if ti.arg != conR {
		conR.logger.Debug("ConsensusReactor changed ...")
		return false
	}
	***/
	conR.logger.Debug("Handle schedule", "scheduling", fn)
	fn()
	return true
}

//////////////////////////////////////////////////////
// update current delegates with new delegates from staking or config file
// keep this standalone method intentionly
func (conR *ConsensusReactor) UpdateCurDelegates() {
	delegates, delegateSize, committeeSize := conR.GetConsensusDelegates()
	conR.allDelegates = delegates
	conR.curDelegates = types.NewDelegateSet(delegates[:delegateSize])
	conR.delegateSize = delegateSize
	conR.committeeSize = uint32(committeeSize)
	names := make([]string, 0)
	for _, d := range conR.curDelegates.Delegates {
		names = append(names, string(d.Name))
	}
	conR.logger.Info("Update curDelegates", "delegateSize", conR.delegateSize, "committeeSize", conR.committeeSize, "names", strings.Join(names, ","))
}

// if best block is moving ahead, and lastkblock is match, we consider there is
// an established committee and proposing
func (conR *ConsensusReactor) CheckEstablishedCommittee(kHeight uint32) bool {
	best := conR.chain.BestBlock()
	bestHeight := best.Header().Number()
	lastKBlockHeight := best.Header().LastKBlockHeight()
	if (bestHeight > kHeight) && ((bestHeight - kHeight) >= 5) && (kHeight == lastKBlockHeight) {
		return true
	}
	return false
}

func (conR *ConsensusReactor) JoinEstablishedCommittee(kBlock *block.Block, replay bool) {
	var nonce uint64
	var info *powpool.PowBlockInfo
	kBlockHeight := kBlock.Header().Number()
	if kBlock.Header().Number() == 0 {
		nonce = genesis.GenesisNonce
		info = powpool.GetPowGenesisBlockInfo()
	} else {
		nonce = kBlock.KBlockData.Nonce
		info = powpool.NewPowBlockInfoFromPosKBlock(kBlock)
	}
	epoch := conR.chain.BestBlock().GetBlockEpoch()
	conR.logger.Info("Received a nonce ...", "nonce", nonce, "kBlockHeight", kBlockHeight, "replay", replay, "epoch", epoch)

	conR.curNonce = nonce

	role, inCommittee := conR.NewValidatorSetByNonce(nonce)

	conR.inCommittee = inCommittee

	if inCommittee {
		conR.logger.Info("I am in committee!!!")
		pool := powpool.GetGlobPowPoolInst()
		pool.Wash()
		pool.InitialAddKframe(info)
		conR.logger.Info("PowPool initial added kblock", "kblock height", kBlock.Header().Number(), "powHeight", info.PowHeight)

		if replay == true {
			//kblock is already added to pool, should start with next one
			startHeight := info.PowHeight + 1
			conR.logger.Info("Replay", "replay from powHeight", startHeight)
			pool.ReplayFrom(int32(startHeight))
		}
		conR.inCommittee = true
		inCommitteeGauge.Set(1)
	} else {
		conR.inCommittee = false
		inCommitteeGauge.Set(0)
		conR.logger.Info("I am NOT in committee!!!", "nonce", nonce)
	}

	if role == CONSENSUS_COMMIT_ROLE_LEADER || role == CONSENSUS_COMMIT_ROLE_VALIDATOR {

		conR.logger.Info("I am committee validator for nonce!", "nonce", nonce)

		conR.NewCommitteeInit(kBlockHeight, nonce, replay)
		conR.updateCurEpoch(epoch)
		conR.NewValidatorSetByNonce(nonce)
		consentBlock, err := conR.chain.GetTrunkBlock(kBlockHeight + 1)
		if err != nil {
			fmt.Println("could not get committee info, stop right now")
			return
		}
		// recover actual committee from consent block
		committeeInfo := consentBlock.CommitteeInfos
		leaderIndex := committeeInfo.CommitteeInfo[0].CSIndex
		conR.UpdateActualCommittee(leaderIndex, conR.config)

		if !conR.config.InitCfgdDelegates {
			// verify in committee status with consent block
			// if delegates are obtained from staking
			myself := conR.curCommittee.Validators[conR.curCommitteeIndex]
			myEcdsaPKBytes := crypto.FromECDSAPub(&myself.PubKey)
			inCommitteeVerified := false
			for _, v := range committeeInfo.CommitteeInfo {
				if bytes.Compare(v.PubKey, myEcdsaPKBytes) == 0 {
					inCommitteeVerified = true
					break
				}
			}
			if inCommitteeVerified == false {
				conR.logger.Error("committee info in consent block doesn't contain myself as a member, stop right now")
				return
			}
		}

		err = conR.startPacemaker(!replay, PMModeCatchUp)
		if err != nil {
			fmt.Println("could not start pacemaker, error:", err)
		}

	} else if role == CONSENSUS_COMMIT_ROLE_NONE {
		// even though it is not committee, still initialize NewCommittee for next
		conR.NewCommitteeInit(kBlockHeight, nonce, replay)
	}
}

// Consensus module handle received nonce from kblock
func (conR *ConsensusReactor) ConsensusHandleReceivedNonce(kBlockHeight uint32, nonce, epoch uint64, replay bool) {
	conR.logger.Info("Received a nonce ...", "nonce", nonce, "kBlockHeight", kBlockHeight, "replay", replay, "epoch", epoch)

	//conR.lastKBlockHeight = kBlockHeight
	conR.curNonce = nonce

	role, inCommittee := conR.NewValidatorSetByNonce(nonce)

	conR.inCommittee = inCommittee

	if inCommittee {
		conR.logger.Info("I am in committee!!!")

		info := &powpool.PowBlockInfo{}
		if kBlockHeight == 0 {
			info = powpool.GetPowGenesisBlockInfo()
		} else {
			kblock, _ := conR.chain.GetTrunkBlock(uint32(kBlockHeight))
			info = powpool.NewPowBlockInfoFromPosKBlock(kblock)
		}
		pool := powpool.GetGlobPowPoolInst()
		pool.Wash()
		pool.InitialAddKframe(info)
		conR.logger.Info("PowPool initial added kblock", "kblock height", kBlockHeight, "powHeight", info.PowHeight)

		if replay == true {
			//kblock is already added to pool, should start with next one
			startHeight := info.PowHeight + 1
			conR.logger.Info("Replay", "replay from powHeight", startHeight)
			pool.ReplayFrom(int32(startHeight))
		}
		conR.inCommittee = true
		inCommitteeGauge.Set(1)
	} else {
		conR.inCommittee = false
		inCommitteeGauge.Set(0)
		conR.logger.Info("I am NOT in committee!!!", "nonce", nonce)
	}

	// hotstuff: check the mechnism:
	// 1) send moveNewRound (with signature) to new leader. if new leader receives majority signature, then send out announce.
	if role == CONSENSUS_COMMIT_ROLE_LEADER {
		conR.logger.Info("I am committee leader for nonce!", "nonce", nonce)
		conR.updateCurEpoch(epoch)

		conR.exitConsensusLeader(conR.curEpoch)
		conR.enterConsensusLeader(conR.curEpoch)

		// no replay case, the last block must be kblock!
		if replay == false && conR.curHeight != kBlockHeight {
			conR.logger.Error("the best block is not kblock", "curHeight", conR.curHeight, "kblock Height", kBlockHeight)
			return
		}

		conR.NewCommitteeInit(kBlockHeight, nonce, replay)

		//wait for majority
	} else if role == CONSENSUS_COMMIT_ROLE_VALIDATOR {
		conR.logger.Info("I am committee validator for nonce!", "nonce", nonce)

		// no replay case, the last block must be kblock!
		if replay == false && conR.curHeight != kBlockHeight {
			conR.logger.Error("the best block is not kblock", "curHeight", conR.curHeight, "kblock Height", kBlockHeight)
			return
		}

		conR.NewCommitteeInit(kBlockHeight, nonce, replay)
		newCommittee := conR.newCommittee
		nl := newCommittee.Committee.Validators[int(newCommittee.Round)%len(newCommittee.Committee.Validators)]
		leader := newConsensusPeer(nl.Name, nl.NetAddr.IP, nl.NetAddr.Port, conR.magic)
		leaderPubKey := nl.PubKey
		conR.sendNewCommitteeMessage(leader, leaderPubKey, newCommittee.KblockHeight,
			newCommittee.Nonce, newCommittee.Round)
		conR.NewCommitteeTimerStart()
	} else if role == CONSENSUS_COMMIT_ROLE_NONE {
		// even though it is not committee, still initialize NewCommittee for next
		conR.NewCommitteeInit(kBlockHeight, nonce, replay)
	}
	return
}

// newCommittee: start with new committee or not
// true --- with new committee, round = 0, best block is kblock.
// false ---replay mode, round continues with BestQC.QCRound. best block is mblock
// XXX: we assume the peers have the bestQC, if they don't ...
func (conR *ConsensusReactor) startPacemaker(newCommittee bool, mode PMMode) error {
	// 1. bestQC height == best block height
	// 2. newCommittee is true, best block is kblock
	for i := 0; i < 3; i++ {
		conR.chain.UpdateBestQC(nil, chain.None)
		bestQC := conR.chain.BestQC()
		bestBlock := conR.chain.BestBlock()
		conR.logger.Info("Checking the QCHeight and Block height...", "QCHeight", bestQC.QCHeight, "bestHeight", bestBlock.Header().Number())
		if bestQC.QCHeight != bestBlock.Header().Number() {
			com := comm.GetGlobCommInst()
			if com == nil {
				conR.logger.Error("get global comm inst failed")
				return errors.New("pacemaker does not started")
			}
			com.TriggerSync()
			conR.logger.Warn("bestQC and bestBlock Height are not match ...")
			<-time.NewTimer(1 * time.Second).C
		} else {
			break
		}
	}
	conR.chain.UpdateBestQC(nil, chain.None)
	bestQC := conR.chain.BestQC()
	bestBlock := conR.chain.BestBlock()
	if bestQC.QCHeight != bestBlock.Header().Number() {
		conR.logger.Error("bestQC and bestBlock still not match, Action (start pacemaker) cancelled ...")
		return nil
	}

	conR.logger.Info("startConsensusPacemaker", "QCHeight", bestQC.QCHeight, "bestHeight", bestBlock.Header().Number())
	conR.csPacemaker.Start(newCommittee, mode)
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

// for committee
// The voteNum does not include leader himself
func LeaderMajorityTwoThird(voterNum, committeeSize uint32) bool {
	if (voterNum < 0) || (committeeSize <= 1) {
		fmt.Println("MajorityTwoThird, inputs out of range")
		return false
	}
	// Examples
	// committeeSize= 2 twoThirds= 1
	// committeeSize= 3 twoThirds= 1
	// committeeSize= 4 twoThirds= 2
	// committeeSize= 5 twoThirds= 3
	// committeeSize= 6 twoThirds= 3
	var twoThirds float64
	if committeeSize == 2 {
		twoThirds = 1
	} else {
		twoThirds = math.Ceil(float64(committeeSize)*2/3) - 1
	}
	if float64(voterNum) >= twoThirds {
		return true
	}
	return false
}

//============================================================================
//============================================================================
// Testing support code
//============================================================================
//============================================================================

func UserHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func (conR *ConsensusReactor) convertFromIntern(interns []*types.DelegateIntern) []*types.Delegate {
	ret := []*types.Delegate{}
	for _, in := range interns {
		pubKey, blsPub := conR.splitPubKey(string(in.PubKey))
		d := &types.Delegate{
			Name:        in.Name,
			Address:     in.Address,
			PubKey:      *pubKey,
			BlsPubKey:   *blsPub,
			VotingPower: in.VotingPower,
			NetAddr:     in.NetAddr,
			Commission:  in.Commission,
			DistList:    in.DistList,
		}
		ret = append(ret, d)
	}

	return ret
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

func (conR *ConsensusReactor) LoadBlockBytes(num uint32) []byte {
	raw, err := conR.chain.GetTrunkBlockRaw(num)
	if err != nil {
		fmt.Print("Error load raw block: ", err)
		return []byte{}
	}
	return raw[:]
}

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

func calcCommitteeSize(delegateSize int, config ConsensusConfig) (int, int) {
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
	if forceDelegates == true {
		delegates = conR.config.InitDelegates
		conR.sourceDelegates = fromDelegatesFile
		fmt.Println("Load delegates from delegates.json")
	} else {
		delegatesIntern, err := staking.GetInternalDelegateList()
		delegates = conR.convertFromIntern(delegatesIntern)
		fmt.Println("Load delegates from staking candidates")
		conR.sourceDelegates = fromStaking
		if err != nil || len(delegates) < conR.config.MinCommitteeSize {
			delegates = conR.config.InitDelegates
			fmt.Println("Load delegates from delegates.json as fallback, error loading staking candiates")
			conR.sourceDelegates = fromStaking
		}
	}

	delegateSize, committeeSize := calcCommitteeSize(len(delegates), conR.config)
	conR.allDelegates = delegates
	conR.logger.Info("Loaded delegates", "delegateSize", delegateSize, "committeeSize", committeeSize)
	PrintDelegates(delegates[:delegateSize])
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

// used for probe API
func (conR *ConsensusReactor) IsPacemakerRunning() bool {
	if conR.csPacemaker == nil {
		return false
	}
	return !conR.csPacemaker.IsStopped()
}

func (conR *ConsensusReactor) IsCommitteeMember() bool {
	return conR.inCommittee
}

func (conR *ConsensusReactor) GetQCHigh() *block.QuorumCert {
	if conR.csPacemaker == nil {
		return nil
	}
	if conR.csPacemaker.QCHigh == nil {
		return nil
	}
	return conR.csPacemaker.QCHigh.QC
}
