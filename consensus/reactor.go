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
	"github.com/meterio/meter-pov/logdb"
	"github.com/meterio/meter-pov/meter"
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
	committeeSize uint32
	knownIPs      map[string]string
	curDelegates  []*types.Delegate // current delegates list

	committee            []*types.Validator // current committee that I start meter into
	hardCommittee        []*types.Validator // committee that was supposed to issue the QC
	bootstrapCommittee11 []*types.Validator // bootstrap committee of size 11
	bootstrapCommittee5  []*types.Validator // bootstrap committee of size 5

	committeeIndex   uint32
	inCommittee      bool
	delegateSource   int
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
		knownIPs:     make(map[string]string),

		inQueue:    NewIncomingQueue(),
		outQueue:   NewOutgoingQueue(),
		EpochEndCh: make(chan EpochEndInfo, CHAN_DEFAULT_BUF_SIZE),

		blsCommon: blsCommon,
		myPrivKey: *privKey,
		myPubKey:  *pubKey,
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

	// initialize consensus common
	r.logger.Info("my keys", "pubkey", b64.RawStdEncoding.EncodeToString(crypto.FromECDSAPub(pubKey)), "privkey", b64.RawStdEncoding.EncodeToString(crypto.FromECDSA(privKey)))

	r.bootstrapCommittee11 = r.bootstrapCommitteeSize11()
	r.bootstrapCommittee5 = r.bootstrapCommitteeSize5()

	// committee info is stored in the first of Mblock after Kblock
	r.UpdateCurEpoch()

	// initialize pacemaker
	r.pacemaker = NewPacemaker(r)

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

func (r *Reactor) VerifyComboPubKey(delegates []*types.Delegate) {
	for _, d := range delegates {
		if bytes.Equal(crypto.FromECDSAPub(&d.PubKey), crypto.FromECDSAPub(&r.myPubKey)) {
			if r.GetComboPubKey() != d.GetComboPubkey() {
				fmt.Println("My Combo PubKey: ", r.GetComboPubKey())
				fmt.Println("Combo PubKey in delegate list:", d.GetComboPubkey())
				panic("ECDSA key found in delegate list, but comboPubKey mismatch")
			}

			blsCommonSystem := r.blsCommon.GetSystem()

			myBlsPubKey := blsCommonSystem.PubKeyToBytes(r.blsCommon.PubKey)
			delegateBlsPubKey := blsCommonSystem.PubKeyToBytes(d.BlsPubKey)

			if !bytes.Equal(myBlsPubKey, delegateBlsPubKey) {
				panic("ECDSA key found in delegate list, but BLS key mismatch, probably wrong info registered in candidate")
			}
		}
	}
}

func (r *Reactor) sortBootstrapCommitteeByNonce(nonce uint64) {
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)

	r.logger.Info("cal bootstrap committee", "nonce", nonce)
	// sort bootstrap committee size of 11
	for _, v := range r.bootstrapCommittee11 {
		v.SortKey = crypto.Keccak256(append(crypto.FromECDSAPub(&v.PubKey), buf...))
	}
	sort.SliceStable(r.bootstrapCommittee11, func(i, j int) bool {
		return (bytes.Compare(r.bootstrapCommittee11[i].SortKey, r.bootstrapCommittee11[j].SortKey) <= 0)
	})

	// sort bootstrap committee size of 5
	for _, v := range r.bootstrapCommittee5 {
		v.SortKey = crypto.Keccak256(append(crypto.FromECDSAPub(&v.PubKey), buf...))
	}
	sort.SliceStable(r.bootstrapCommittee5, func(i, j int) bool {
		return (bytes.Compare(r.bootstrapCommittee5[i].SortKey, r.bootstrapCommittee5[j].SortKey) <= 0)
	})
}

// it is used for temp calculate committee set by a given nonce in the fly.
// also return the committee
func (r *Reactor) calcCommitteeByNonce(name string, delegates []*types.Delegate, nonce uint64) ([]*types.Delegate, []*types.Validator, uint32, bool) {
	delegateSize, committeeSize := calcCommitteeSize(len(delegates), r.config)
	actualDelegates := delegates[:delegateSize]

	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)

	validators := make([]*types.Validator, 0)
	for _, d := range actualDelegates {
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
		validators = append(validators, v)
	}
	r.logger.Info(fmt.Sprintf("cal %s committee", name), "delegateSize", len(actualDelegates), "committeeSize", committeeSize, "nonce", nonce)

	sort.SliceStable(validators, func(i, j int) bool {
		return (bytes.Compare(validators[i].SortKey, validators[j].SortKey) <= 0)
	})

	committee := validators[:committeeSize]
	// the full list is stored in currCommittee, sorted.
	// To become a validator (real member in committee), must repond the leader's
	// announce. Validators are stored in r.conS.Vlidators
	if len(committee) > 0 {
		for i, val := range committee {
			if bytes.Equal(crypto.FromECDSAPub(&val.PubKey), crypto.FromECDSAPub(&r.myPubKey)) {

				return actualDelegates, committee, uint32(i), true
			}
		}
	} else {
		r.logger.Error("committee is empty, potential error config with delegates.json", "delegates", len(delegates))
	}

	return actualDelegates, committee, 0, false
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

	_, err = r.UpdateCurEpoch()
	if err != nil {
		return err
	}
	r.logger.Info("prepare env for pacemaker", "nonce", r.curNonce, "bestK", bestKBlock.Number(), "bestIsKBlock", bestIsKBlock, "epoch", r.curEpoch)

	var info *powpool.PowBlockInfo
	if bestKBlock.Number() == 0 {
		info = powpool.GetPowGenesisBlockInfo()
	} else {
		info = powpool.NewPowBlockInfoFromPosKBlock(bestKBlock)
	}
	r.logger.Info("Powpool prepare to add kframe, and notify PoW chain to pick head", "powHeight", info.PowHeight, "powRawBlock", hex.EncodeToString(info.PowRaw))
	pool := powpool.GetGlobPowPoolInst()
	// pool.Wash()
	pool.InitialAddKframe(info)
	r.logger.Info("Powpool initial added kframe", "bestK", bestKBlock.Number(), "powHeight", info.PowHeight)
	if r.inCommittee {
		//kblock is already added to pool, should start with next one
		startHeight := info.PowHeight + 1
		r.logger.Info("Replay pow blocks", "fromHeight", startHeight)
		pool.ReplayFrom(int32(startHeight))
	}

	return nil
}

func (r *Reactor) verifyInCommittee() bool {
	best := r.chain.BestBlock()
	if r.delegateSource == fromDelegatesFile {
		return true
	}

	if !best.IsKBlock() && best.Number() != 0 {
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

func (r *Reactor) combinePubKey(ecdsaPub *ecdsa.PublicKey, blsPub *bls.PublicKey) string {
	ecdsaPubBytes := crypto.FromECDSAPub(ecdsaPub)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)

	blsPubBytes := r.blsCommon.GetSystem().PubKeyToBytes(*blsPub)
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	return strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
}

func (r *Reactor) GetComboPubKey() string {
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
func (r *Reactor) GetConsensusDelegates() ([]*types.Delegate, []*types.Delegate) {
	forceDelegates := r.config.InitCfgdDelegates

	// special handle for flag --init-configured-delegates
	var delegates []*types.Delegate
	var err error
	if forceDelegates {
		delegates = r.config.InitDelegates
		r.delegateSource = fromDelegatesFile
		r.peakFirst3Delegates("Loaded delegates from file", delegates)
	} else {
		delegates, err = r.getDelegatesFromStaking()
		r.delegateSource = fromStaking
		r.peakFirst3Delegates("Loaded delegates from staking", delegates)
		if err != nil || len(delegates) < r.config.MinCommitteeSize {
			delegates = r.config.InitDelegates
			r.delegateSource = fromDelegatesFile
			r.peakFirst3Delegates("Loaded delegates from file as fallback", delegates)
		}
	}

	for _, d := range delegates {
		r.knownIPs[d.NetAddr.IP.String()] = string(d.Name)
	}
	stakingDelegates, _ := r.getDelegatesFromStaking()

	return delegates, stakingDelegates
}

func (r *Reactor) peakFirst3Delegates(hint string, delegates []*types.Delegate) {
	first3Names := make([]string, 0)
	for i := 0; i < len(delegates) && i < 3; i++ {
		d := delegates[i]
		name := string(d.Name)
		first3Names = append(first3Names, name)
	}

	r.logger.Info(hint, "first3", strings.Join(first3Names, ","))
}

func (r *Reactor) getNameByIP(ip net.IP) string {
	if name, exist := r.knownIPs[ip.String()]; exist {
		return name
	}
	return ""
}

func (r *Reactor) UpdateCurEpoch() (bool, error) {
	bestK, err := r.chain.BestKBlock()
	if err != nil {
		r.logger.Error("could not get bestK", "err", err)
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
	if epoch >= r.curEpoch && r.curNonce != nonce {
		r.logger.Info(fmt.Sprintf("Entering epoch %v", epoch), "lastEpoch", r.curEpoch)
		r.logger.Info("---------------------------------------------------------")

		//initialize Delegates
		delegates, stakingDelegates := r.GetConsensusDelegates()

		// notice: this will panic if ECDSA key matches but BLS doesn't
		r.VerifyComboPubKey(delegates)

		r.curDelegates, r.committee, r.committeeIndex, r.inCommittee = r.calcCommitteeByNonce("current", delegates, nonce)
		r.committeeSize = uint32(len(r.committee))
		if r.delegateSource == fromStaking {
			r.hardCommittee = r.committee
		} else {
			_, r.hardCommittee, _, _ = r.calcCommitteeByNonce("hard", stakingDelegates, nonce)
		}
		r.sortBootstrapCommitteeByNonce(nonce)
		r.PrintCommittee()

		// update nonce
		r.curNonce = nonce

		if r.inCommittee {
			myAddr := r.committee[r.committeeIndex].NetAddr
			myName := r.committee[r.committeeIndex].Name

			r.logger.Info("I'm IN committee !!!", "myName", myName, "myIP", myAddr.IP.String())
			inCommitteeGauge.Set(1)
			verified := r.verifyInCommittee()
			if !verified {
				//FIXME: fix this
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
		r.logger.Info("---------------------------------------------------------")
		r.logger.Info(fmt.Sprintf("Entered epoch %d", r.curEpoch), "lastEpoch", lastEpoch)
		r.logger.Info("---------------------------------------------------------")
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
	var valid bool
	var err error
	if escortQC.VoterBitArray().Size() == len(r.bootstrapCommittee11) {
		// validate with bootstrap committee of size 11
		start := time.Now()
		valid, err = b.VerifyQC(escortQC, r.blsCommon, r.bootstrapCommittee11)
		if err != nil {
			r.logger.Error("QC validate failed with bootstrap committee size 11", "err", err)
		}
		r.logger.Info("Validate QC with bootstrap committee size 11", "valid", valid, "elapsed", meter.PrettyDuration(time.Since(start)))
		if valid {
			return true
		}
	} else if escortQC.VoterBitArray().Size() == len(r.bootstrapCommittee5) {
		// validate with bootstrap committee of size 5
		start := time.Now()
		valid, err = b.VerifyQC(escortQC, r.blsCommon, r.bootstrapCommittee5)
		if err != nil {
			r.logger.Error("QC validate failed with bootstrap committee size 5", "err", err)
		}
		r.logger.Info("Validate QC with bootstrap committee size 5", "valid", valid, "elapsed", meter.PrettyDuration(time.Since(start)))
		if valid {
			return true
		}
	}
	if r.delegateSource != fromStaking && escortQC.VoterBitArray().Size() == len(r.hardCommittee) {
		// validate with hard committee
		start := time.Now()
		valid, err = b.VerifyQC(escortQC, r.blsCommon, r.hardCommittee)
		if err != nil {
			r.logger.Error("QC validate failed with hard committee", "err", err)
		}
		if valid {
			return true
		}
		r.logger.Info("Validate QC with hard committee", "committeeSize", len(r.hardCommittee), "valid", valid, "elapsed", meter.PrettyDuration(time.Since(start)))
	}

	// validate with current committee
	start := time.Now()
	valid, err = b.VerifyQC(escortQC, r.blsCommon, r.committee)
	if err != nil {
		r.logger.Error("QC validate failed", "err", err)
	}

	r.logger.Info("Validate QC with current committee", "valid", valid, "elapsed", meter.PrettyDuration(time.Since(start)))
	return valid
}

func (r *Reactor) peakCommittee(committee []*types.Validator) string {
	s := make([]string, 0)
	if len(committee) > 6 {
		for index, val := range committee[:3] {
			s = append(s, fmt.Sprintf("#%-4v %v", index, val.String()))
		}
		s = append(s, "...")
		for index, val := range committee[len(committee)-3:] {
			s = append(s, fmt.Sprintf("#%-4v %v", index+len(committee)-3, val.String()))
		}
	} else {
		for index, val := range committee {
			s = append(s, fmt.Sprintf("#%-2v %v", index, val.String()))
		}
	}
	return strings.Join(s, "\n")
}

func (r *Reactor) PrintCommittee() {
	fmt.Printf("Current Committee (%d):\n%s\n", len(r.committee), r.peakCommittee(r.committee))

	if r.delegateSource != fromStaking {
		fmt.Printf("Hard Committee (%d):\n%s\n", len(r.hardCommittee), r.peakCommittee(r.hardCommittee))
	}

	// fmt.Println("Bootstrap11 Committee:\n" + r.peakCommittee(r.bootstrapCommittee11))
}
