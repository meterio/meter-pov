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
	"errors"
	"fmt"
	"io/ioutil"
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
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/packer"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/state"
	"github.com/meterio/meter-pov/txpool"
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
	txpool       *txpool.TxPool
	comm         *comm.Communicator
	packer       *packer.Packer
	chain        *chain.Chain
	logDB        *logdb.LogDB
	stateCreator *state.Creator
	logger       log15.Logger
	config       ReactorConfig
	SyncDone     bool

	// copy of master/node
	myPubKey  ecdsa.PublicKey  // this is my public identification !!
	myPrivKey ecdsa.PrivateKey // copy of private key

	// still references above consensuStae, reactor if this node is
	// involved the consensus
	delegateSize     int // global constant, current available delegate size.
	committeeSize    uint32
	allDelegates     []*types.Delegate
	curDelegates     []*types.Delegate  // current delegates list
	committee        []*types.Validator // Real committee, should be subset of curCommittee if someone is offline.
	committeeIndex   uint32
	inCommittee      bool
	sourceDelegates  int
	lastKBlockHeight uint32
	curNonce         uint64
	curEpoch         uint64

	blsCommon *types.BlsCommon //this must be allocated as validator
	pacemaker *Pacemaker

	EpochEndCh chan EpochEndInfo // this channel for kblock notify from node module.

	magic [4]byte

	inQueue  *IncomingQueue
	outQueue *OutgoingQueue
}

// NewConsensusReactor returns a new Reactor with config
func NewConsensusReactor(ctx *cli.Context, chain *chain.Chain, logDB *logdb.LogDB, comm *comm.Communicator, txpool *txpool.TxPool, packer *packer.Packer, state *state.Creator, privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey, magic [4]byte, blsCommon *types.BlsCommon, initDelegates []*types.Delegate) *Reactor {
	prometheus.Register(pmRoundGauge)
	prometheus.Register(curEpochGauge)
	prometheus.Register(lastKBlockHeightGauge)
	prometheus.Register(blocksCommitedCounter)
	prometheus.Register(inCommitteeGauge)
	prometheus.Register(pmRoleGauge)

	r := &Reactor{
		comm:         comm,
		txpool:       txpool,
		packer:       packer,
		chain:        chain,
		logDB:        logDB,
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

	// initialize consensus common
	r.blsCommon = blsCommon

	// initialize pacemaker
	r.pacemaker = NewPacemaker(r)

	// committee info is stored in the first of Mblock after Kblock
	r.UpdateCurEpoch()

	r.myPrivKey = *privKey
	r.myPubKey = *pubKey

	return r
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (r *Reactor) OnStart(ctx context.Context) error {

	go r.outQueue.Start(ctx)

	select {
	case <-ctx.Done():
		r.logger.Warn("stop reactor due to context end")
		return nil
	case <-r.comm.Synced():
		r.logger.Info("sync is done, start pacemaker ...")
		r.pacemaker.Regulate()
	}

	return nil
}

func (r *Reactor) GetLastKBlockHeight() uint32 {
	return r.lastKBlockHeight
}

// get the specific round proposer
func (r *Reactor) getRoundProposer(round uint32) *types.Validator {
	size := len(r.committee)
	if size == 0 {
		return &types.Validator{}
	}
	return r.committee[int(round)%size]
}

func (r *Reactor) amIRoundProproser(round uint32) bool {
	p := r.getRoundProposer(round)
	return bytes.Equal(p.PubKeyBytes, crypto.FromECDSAPub(&r.myPubKey))
}

func (r *Reactor) VerifyBothPubKey() {
	for _, d := range r.curDelegates {
		if bytes.Equal(crypto.FromECDSAPub(&d.PubKey), crypto.FromECDSAPub(&r.myPubKey)) == true {
			if r.GetCombinePubKey() != d.GetInternCombinePubKey() {
				fmt.Println("Combine PubKey: ", r.GetCombinePubKey())
				fmt.Println("Intern Combine PubKey: ", d.GetInternCombinePubKey())
				panic("ECDSA key found in delegate list, but combinePubKey mismatch")
			}

			blsCommonSystem := r.blsCommon.GetSystem()

			myBlsPubKey := blsCommonSystem.PubKeyToBytes(r.blsCommon.PubKey)
			delegateBlsPubKey := blsCommonSystem.PubKeyToBytes(d.BlsPubKey)

			if !bytes.Equal(myBlsPubKey, delegateBlsPubKey) {
				panic("ECDSA key found in delegate list, but related BLS key mismatch with delegate, probably wrong info in candidate")
			}
		}
	}
}

// create validatorSet by a given nonce. return by my self role
func (r *Reactor) UpdateCurCommitteeByNonce(nonce uint64) bool {
	committee, index, inCommittee := r.calcCommitteeByNonce(nonce)
	r.committee = committee
	r.committeeIndex = uint32(index)
	r.inCommittee = inCommittee
	if inCommittee {
		myAddr := committee[index].NetAddr
		myName := committee[index].Name
		r.logger.Info("New committee calculated", "index", index, "myName", myName, "myIP", myAddr.IP.String())
	} else {
		// FIXME: find a better way
		r.logger.Info("New committee calculated")
	}
	fmt.Println("Committee members in order:")
	fmt.Println(committee)

	return inCommittee
}

// it is used for temp calculate committee set by a given nonce in the fly.
// also return the committee
func (r *Reactor) calcCommitteeByNonce(nonce uint64) ([]*types.Validator, int, bool) {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)

	vals := make([]*types.Validator, 0)
	for _, d := range r.curDelegates {
		v := &types.Validator{
			Name:           string(d.Name),
			Address:        d.Address,
			PubKey:         d.PubKey,
			PubKeyBytes:    crypto.FromECDSAPub(&d.PubKey),
			BlsPubKey:      d.BlsPubKey,
			BlsPubKeyBytes: r.blsCommon.GetSystem().PubKeyToBytes(d.BlsPubKey),
			VotingPower:    d.VotingPower,
			NetAddr:        d.NetAddr,
			SortKey:        crypto.Keccak256(append(crypto.FromECDSAPub(&d.PubKey), buf...)),
		}
		vals = append(vals, v)
	}
	r.logger.Info("cal committee", "delegates", len(r.curDelegates), "committeeSize", r.committeeSize, "vals", len(vals))

	sort.SliceStable(vals, func(i, j int) bool {
		return (bytes.Compare(vals[i].SortKey, vals[j].SortKey) <= 0)
	})

	vals = vals[:r.committeeSize]
	// the full list is stored in currCommittee, sorted.
	// To become a validator (real member in committee), must repond the leader's
	// announce. Validators are stored in r.conS.Vlidators
	if len(vals) > 0 {
		for i, val := range vals {
			if bytes.Equal(crypto.FromECDSAPub(&val.PubKey), crypto.FromECDSAPub(&r.myPubKey)) {

				return vals, i, true
			}
		}
	}

	r.logger.Error("VALIDATOR SET is empty, potential error config with delegates.json", "delegates", len(r.curDelegates))
	return vals, 0, false
}

func (r *Reactor) GetMyNetAddr() types.NetAddress {
	if r.committee != nil && len(r.committee) > 0 {
		return r.committee[r.committeeIndex].NetAddr
	}
	return types.NetAddress{IP: net.IP{}, Port: 0}
}

func (r *Reactor) GetMyName() string {
	if r.committee != nil && len(r.committee) > 0 {
		return r.committee[r.committeeIndex].Name
	}
	return "unknown"
}

func (r *Reactor) PrepareEnvForPacemaker() error {

	bestKBlock, err := r.chain.BestKBlock()
	if err != nil {
		fmt.Println("could not get best KBlock", err)
		return errors.New("could not get best KBlock")
	}
	bestBlock := r.chain.BestBlock()
	bestIsKBlock := bestBlock.IsKBlock() || bestBlock.Header().Number() == 0

	updated, err := r.UpdateCurEpoch()
	if err != nil {
		return err
	}
	r.logger.Info("prepare env for pacemaker", "nonce", r.curNonce, "bestK", bestKBlock.Number(), "bestIsKBlock", bestIsKBlock, "epoch", r.curEpoch)

	if updated {
		var info *powpool.PowBlockInfo
		if bestKBlock.Number() == 0 {
			info = powpool.GetPowGenesisBlockInfo()
		} else {
			info = powpool.NewPowBlockInfoFromPosKBlock(bestKBlock)
		}
		// r.logger.Info("Powpool prepare to add kframe, and notify PoW chain to pick head", "powHeight", info.PowHeight, "powRawBlock", hex.EncodeToString(info.PowRaw))
		pool := powpool.GetGlobPowPoolInst()
		pool.Wash()
		pool.InitialAddKframe(info)
		r.logger.Info("Powpool initial added kframe", "bestK", bestKBlock.Number(), "powHeight", info.PowHeight)
		if r.inCommittee && bestIsKBlock {
			//kblock is already added to pool, should start with next one
			startHeight := info.PowHeight + 1
			r.logger.Info("Replay pow blocks", "fromHeight", startHeight)
			pool.ReplayFrom(int32(startHeight))
		}
	}

	return nil
}

func (r *Reactor) verifyInCommittee() bool {
	best := r.chain.BestBlock()
	if r.config.InitCfgdDelegates {
		return true
	}

	if !r.config.InitCfgdDelegates && !best.IsKBlock() && best.Number() != 0 {
		consentBlock, err := r.chain.GetTrunkBlock(best.LastKBlockHeight() + 1)
		if err != nil {
			fmt.Println("could not get committee info:", err)
			return false
		}
		// recover actual committee from consent block
		committeeInfo := consentBlock.CommitteeInfos

		myself := r.committee[r.committeeIndex]
		myEcdsaPKBytes := crypto.FromECDSAPub(&myself.PubKey)
		inCommitteeVerified := false
		for _, v := range committeeInfo.CommitteeInfo {
			if bytes.Equal(v.PubKey, myEcdsaPKBytes) {
				inCommitteeVerified = true
				break
			}
		}
		return inCommitteeVerified
	}
	return false
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

func (r *Reactor) UpdateCurEpoch() (bool, error) {
	bestK, err := r.chain.BestKBlock()
	if err != nil {
		return false, errors.New("could not get best KBlock")
	}

	var nonce uint64
	epoch := uint64(0)
	if bestK.Number() == 0 {
		nonce = genesis.GenesisNonce
	} else {
		nonce = bestK.KBlockData.Nonce
		epoch = bestK.GetBlockEpoch() + 1
	}

	if epoch > r.curEpoch && r.curNonce != nonce {
		//initialize Delegates
		delegates, delegateSize, committeeSize := r.GetConsensusDelegates()
		r.allDelegates = delegates
		r.curDelegates = delegates[:delegateSize]
		r.delegateSize = delegateSize
		r.committeeSize = uint32(committeeSize)

		// notice: this will panic if ECDSA key matches but BLS doesn't
		r.VerifyBothPubKey()

		// update nonce
		r.curNonce = nonce
		r.inCommittee = r.UpdateCurCommitteeByNonce(nonce)
		if r.inCommittee {
			r.logger.Info("I am in committee!!!")
			inCommitteeGauge.Set(1)
			verified := r.verifyInCommittee()
			if !verified {
				r.logger.Error("committee info in consent block doesn't contain myself as a member, stop right now")
				return false, errors.New("in committee but not verified")
			}
		} else {
			r.logger.Info("I'm NOT in committee")
			inCommitteeGauge.Set(0)
		}
		r.lastKBlockHeight = bestK.Number()
		lastKBlockHeightGauge.Set(float64(bestK.Number()))

		lastEpoch := r.curEpoch
		r.curEpoch = epoch
		curEpochGauge.Set(float64(r.curEpoch))
		r.logger.Info("Epoch updated", "from", lastEpoch, "to", r.curEpoch)
		return true, nil
	}
	return false, nil
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
	if int(signerIndex) >= len(r.committee) {
		r.logger.Warn("actual committee not initialized, skip relay ...", "msg", msg.GetType())
		return
	}
	signer := r.committee[signerIndex]

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

func (r *Reactor) ValidateQC(b *block.Block, escortQC *block.QuorumCert) bool {
	valid, err := b.VerifyQC(escortQC, r.blsCommon, r.committeeSize, r.committee)
	if err != nil {
		r.logger.Error("QC validate failed", "err", err)
	}
	return valid
}
