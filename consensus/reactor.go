package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"

	//"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	amino "github.com/dfinlab/go-amino"
	crypto "github.com/ethereum/go-ethereum/crypto"

	//"github.com/ethereum/go-ethereum/rlp"
	//"github.com/vechain/thor/block"
	//"github.com/ethereum/go-ethereum/p2p"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/chain"
	bls "github.com/vechain/thor/crypto/multi_sig"
	"github.com/vechain/thor/thor"

	//"github.com/vechain/thor/runtime"
	"github.com/vechain/thor/state"
	//"github.com/vechain/thor/tx"
	"github.com/vechain/thor/types"
	//"github.com/vechain/thor/xenv"
	// "github.com/dfinlab/go-zdollar/crypto/ed25519"
	//config "github.com/dfinlab/go-zdollar/config"
	cmn "github.com/vechain/thor/libs/common"
	//tmevents "github.com/dfinlab/go-zdollar/libs/events"
	//"github.com/dfinlab/go-zdollar/libs/log"
	//sm "github.com/dfinlab/go-zdollar/state"
	//tmtime "github.com/dfinlab/go-zdollar/types/time"
)

const (
	maxMsgSize = 1048576 // 1MB; NOTE/TODO: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer = 10000
	votesToContributeToBecomeGoodPeer  = 10000

	COMMITTEE_SIZE = 400  // by default
	DELEGATES_SIZE = 2000 // by default

	// Sign Announce Mesage
	// "Announce Committee Message: Leader <pubkey 64(hexdump 32x2) bytes> CommitteeID <8 (4x2)bytes> Height <16 (8x2) bytes> Round <8(4x2)bytes>
	ANNOUNCE_SIGN_MSG_SIZE = int(110)

	// Sign Propopal Message
	// "Proposal Block Message: Proposer <pubkey 64(32x3)> BlockType <2 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
	PROPOSAL_SIGN_MSG_SIZE = int(100)

	// Sign Notary Announce Message
	// "Announce Notarization Message: Leader <pubkey 64(32x3)> CommitteeID <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
	NOTARY_ANNOUNCE_SIGN_MSG_SIZE = int(120)

	// Sign Notary Block Message
	// "Block Notarization Message: Proposer <pubkey 64(32x3)> BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
	NOTARY_BLOCK_SIGN_MSG_SIZE = int(130)
)

//-----------------------------------------------------------------------------

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	chain        *chain.Chain
	stateCreator *state.Creator

	// copy of master/node
	myPubKey      ecdsa.PublicKey  // this is my public identification !!
	myPrivKey     ecdsa.PrivateKey // copy of private key
	myBeneficiary thor.Address

	// still references above consensuStae, reactor if this node is
	// involved the consensus
	csMode             byte // delegates, committee, other
	delegateSize       int  // global constant
	committeeSize      int
	myDelegatesIndex   int                 // this index will be changed by DelegateSet every time
	curDelegates       *types.DelegateSet  // current delegates list
	curCommittee       *types.ValidatorSet // This is top 400 of delegates by given nonce
	curActualCommittee []CommitteeMember   // Real committee, should be subset of curCommittee if someone is offline.
	curCommitteeIndex  int

	csRoleInitialized uint
	csCommon          *ConsensusCommon //this must be allocated as validator
	csLeader          *ConsensusLeader
	csProposer        *ConsensusProposer
	csValidator       *ConsensusValidator

	// store key states here
	lastKBlockID thor.Bytes32
	//parentBlockID  thor.Bytes32
	curNonce       uint64
	curCommitteeID uint32
	curHeight      int64 // come from parentBlockID first 4 bytes uint32
	curRound       int
	mtx            sync.RWMutex

	// consensus state for new consensus, similar to old conS

	// state changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan consensusMsgInfo
	internalMsgQueue chan consensusMsgInfo
	schedulerQueue   chan consensusTimeOutInfo

	// kBlock data
	KBlockDataQueue chan block.KBlockData
}

// NewConsensusReactor returns a new ConsensusReactor with the given
// consensusState.
func NewConsensusReactor(chain *chain.Chain, state *state.Creator) *ConsensusReactor {
	conR := &ConsensusReactor{
		chain:        chain,
		stateCreator: state,
	}

	//initialize message channel
	conR.peerMsgQueue = make(chan consensusMsgInfo, 100)
	conR.internalMsgQueue = make(chan consensusMsgInfo, 100)
	conR.schedulerQueue = make(chan consensusTimeOutInfo, 100)
	conR.KBlockDataQueue = make(chan block.KBlockData, 100)

	//initialize height/round
	conR.curHeight = int64(chain.BestBlock().Header().Number())
	conR.curRound = 0

	//XXX: Yang: Address it later Get the public key
	//initialize Delegates
	ds := configDelegates()
	conR.curDelegates = types.NewDelegateSet(ds)
	conR.delegateSize = 2  // 10 //DELEGATES_SIZE
	conR.committeeSize = 2 // 4 //COMMITTEE_SIZE

	conR.myPubKey = ds[0].PubKey
	return conR
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (conR *ConsensusReactor) OnStart() error {

	// Start new consensus
	conR.NewConsensusStart()

	// force to receive nonce
	conR.ConsensusHandleReceivedNonce(0, 1001)

	fmt.Println("Consensus started ... ")
	return nil
}

// OnStop implements BaseService by unsubscribing from events and stopping
// state.
func (conR *ConsensusReactor) OnStop() {

	// New consensus
	conR.NewConsensusStop()

}

// SwitchToConsensus switches from fast_sync mode to consensus mode.
// It resets the state, turns off fast_sync, and starts the consensus state-machine
func (conR *ConsensusReactor) SwitchToConsensus(blocksSynced int) {
	//conR.Logger.Info("SwitchToConsensus")
	fmt.Println("SwitchToConsensus")
	fmt.Println("blockSynced", blocksSynced)
	conR.ConsensusHandleReceivedNonce(0, 1001)
}

// String returns a string representation of the ConsensusReactor.
// NOTE: For now, it is just a hard-coded string to avoid accessing unprotected shared variables.
// TODO: improve!
func (conR *ConsensusReactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor" // conR.StringIndented("")
}

/**********************
// StringIndented returns an indented string representation of the ConsensusReactor
func (conR *ConsensusReactor) StringIndented(indent string) string {
	s := "ConsensusReactor{\n"
	s += indent + "  " + conR.conS.StringIndented(indent+"  ") + "\n"
	for _, peer := range conR.Switch.Peers().List() {
		ps := peer.Get(types.PeerStateKey).(*PeerState)
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}
	s += indent + "}"
	return s
}
************************/

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

	//Consensus Message Type
	CONSENSUS_MSG_ANNOUNCE_COMMITTEE = byte(0x01)
	CONSENSUS_MSG_COMMIT_COMMITTEE   = byte(0x02)
	CONSENSUS_MSG_PROPOSAL_BLOCK     = byte(0x03)
	CONSENSUS_MSG_NOTARY_ANNOUNCE    = byte(0x04)
	CONSENSUS_MSG_NOTARY_BLOCK       = byte(0x05)
	CONSENSUS_MSG_VOTE_FOR_PROPOSAL  = byte(0x06)
	CONSENSUS_MSG_VOTE_FOR_NOTARY    = byte(0x07)
	CONSENSUS_MSG_MOVE_NEW_ROUND     = byte(0x08)
)

// CommitteeMember is validator structure + consensus fields
type CommitteeMember struct {
	PubKey      ecdsa.PublicKey
	VotingPower int64
	Accum       int64
	CommitKey   []byte
	NetAddr     types.NetAddress
	CSPubKey    bls.PublicKey
	CSIndex     int
}

// create new committee member
func NewCommitteeMember() *CommitteeMember {
	return &CommitteeMember{}
}

type consensusMsgInfo struct {
	//Msg    ConsensusMessage
	Msg    []byte
	csPeer *ConsensusPeer
}

// set CS mode
func (conR *ConsensusReactor) setCSMode(nMode byte) bool {
	conR.csMode = nMode
	return true
}

func (conR *ConsensusReactor) getCSMode() byte {
	return conR.csMode
}

func (conR *ConsensusReactor) isCSCommittee() bool {
	return (conR.csMode == CONSENSUS_MODE_COMMITTEE)
}

func (conR *ConsensusReactor) isCSDelegates() bool {
	return (conR.csMode == CONSENSUS_MODE_DELEGATE)
}

func (conR *ConsensusReactor) UpdateHeight(height int64) bool {
	fmt.Println("Update conR.curHeight to ", height)
	conR.curHeight = height
	return true
}

func (conR *ConsensusReactor) UpdateRound(round int) bool {
	fmt.Println("Update conR.curRound to ", round)
	conR.curRound = round
	return true
}

// update the Height
func (conR *ConsensusReactor) UpdateHeightRound(height int64, round int) bool {
	if height != 0 {
		conR.curHeight = height
	}

	conR.curRound = round
	return true
}

// after announce/commit, Leader got the actual committee, which is the subset of curCommittee if some committee member offline.
// indexs and pubKeys are not sorted slice, AcutalCommittee must be sorted.
// Only Leader can call this method. indexes do not include the leader itself.
func (conR *ConsensusReactor) UpdateActualCommittee(indexes []int, pubKeys []bls.PublicKey, bitArray *cmn.BitArray) bool {

	if len(indexes) != len(pubKeys) ||
		len(indexes) > conR.committeeSize {
		fmt.Println("failed to update reactor actual committee ...")
		return false
	}

	// Add leader (myself) to the AcutalCommittee
	l := conR.curCommittee.Validators[0]
	cm := CommitteeMember{
		PubKey:      l.PubKey,
		VotingPower: l.VotingPower,
		Accum:       l.Accum,
		CommitKey:   l.CommitKey,
		NetAddr:     l.NetAddr,
		CSPubKey:    conR.csCommon.PubKey, //bls PublicKey
		CSIndex:     conR.curCommitteeIndex,
	}
	conR.curActualCommittee = append(conR.curActualCommittee, cm)

	for i, index := range indexes {
		//sanity check
		if index == -1 ||
			index > conR.committeeSize {
			fmt.Println(i, "index", index)
			continue
		}

		//get validator info
		v := conR.curCommittee.Validators[index]

		cm := CommitteeMember{
			PubKey:      v.PubKey,
			VotingPower: v.VotingPower,
			Accum:       v.Accum,
			CommitKey:   v.CommitKey,
			NetAddr:     v.NetAddr,
			CSPubKey:    pubKeys[i], //bls PublicKey
			CSIndex:     index,
		}

		conR.curActualCommittee = append(conR.curActualCommittee, cm)
	}

	if len(conR.curActualCommittee) == 0 {
		return false
	}

	// Sort them.
	sort.SliceStable(conR.curActualCommittee, func(i, j int) bool {
		return (bytes.Compare(conR.curActualCommittee[i].CommitKey, conR.curActualCommittee[j].CommitKey) <= 0)
	})

	// I am Leader, first one should be myself.
	if conR.curActualCommittee[0].PubKey != conR.myPubKey {
		fmt.Println("I am leader and not in first place of curActualCommittee, must correct ...")
		return false
	}

	return true
}

// get current round proposer
func (conR *ConsensusReactor) getCurrentProposer() CommitteeMember {
	return conR.curActualCommittee[conR.curRound%len(conR.curActualCommittee)]
}

// get the specific round proposer
func (conR *ConsensusReactor) getRoundProposer(round int) CommitteeMember {
	return conR.curActualCommittee[round%len(conR.curActualCommittee)]
}

//create validatorSet by a given nonce. return by my self role
func (conR *ConsensusReactor) NewValidatorSetByNonce(nonce []byte) (uint, bool) {
	//vals []*types.Validator

	vals := make([]*types.Validator, conR.delegateSize)
	for i := 0; i < conR.delegateSize; i++ {

		pubKey := conR.curDelegates.Delegates[i].PubKey
		votePower := int64(1000)
		vals[i] = types.NewValidator(pubKey, votePower)
		vals[i].NetAddr = conR.curDelegates.Delegates[i].NetAddr
		// sorted key is pubkey + nonce ...
		ck := crypto.Keccak256(append(crypto.FromECDSAPub(&pubKey), nonce...))
		vals[i].CommitKey = append(vals[i].CommitKey, ck...)
		fmt.Println(vals[i].CommitKey)
	}

	sort.SliceStable(vals, func(i, j int) bool {
		return (bytes.Compare(vals[i].CommitKey, vals[j].CommitKey) <= 0)
	})

	// the full list is stored in currCommittee, sorted.
	// To become a validator (real member in committee), must repond the leader's
	// announce. Validators are stored in conR.conS.Vlidators

	//conR.conS.Validators = types.NewValidatorSet2(vals[:conR.committeeSize])
	conR.curCommittee = types.NewValidatorSet2(vals[:conR.committeeSize])
	if vals[0].PubKey == conR.myPubKey {
		conR.csMode = CONSENSUS_MODE_COMMITTEE
		conR.curCommitteeIndex = 0
		return CONSENSUS_COMMIT_ROLE_LEADER, true
	}

	for i, val := range vals {
		if val.PubKey == conR.myPubKey {
			conR.csMode = CONSENSUS_MODE_COMMITTEE
			conR.curCommitteeIndex = i
			return CONSENSUS_COMMIT_ROLE_VALIDATOR, true
		}
	}

	conR.csMode = CONSENSUS_MODE_DELEGATE
	conR.curCommitteeIndex = 0
	return CONSENSUS_COMMIT_ROLE_NONE, false
}

func (conR *ConsensusReactor) GetCommitteeMemberIndex(pubKey ecdsa.PublicKey) int {
	for i, v := range conR.curCommittee.Validators {
		if v.PubKey == pubKey {
			return i
		}
	}

	fmt.Println("not in committee", pubKey)
	return -1
}

// Handle received Message
func (conR *ConsensusReactor) handleMsg(mi consensusMsgInfo) {
	conR.mtx.Lock()
	defer conR.mtx.Unlock()

	rawMsg, peer := mi.Msg, mi.csPeer
	fmt.Println("from peer ", peer)
	fmt.Println("receives msg: ", rawMsg)

	msg, err := decodeMsg(rawMsg)
	if err != nil {
		fmt.Println("Error decoding message", "src", peer, "msg", msg, "err", err, "bytes", rawMsg)
		return
	}

	switch msg := msg.(type) {

	// New consensus Messages
	case *AnnounceCommitteeMessage:
		fmt.Println("receives AnnounceCommitteeMessage ...")

		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			fmt.Println("not in validator role, enter validator first ...")
			// if find out we are not in committee, then exit validator
			conR.enterConsensusValidator()
		}

		success := conR.csValidator.ProcessAnnounceCommittee(msg, peer)
		// For ProcessAnnounceCommittee, it is not validator if return is false
		if success == false {
			fmt.Println("process announce failed")
			conR.exitConsensusValidator()
		}

	case *CommitCommitteeMessage:
		fmt.Println("receives CommitCommitteeMessage ...")
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_LEADER) == 0 ||
			(conR.csLeader == nil) {
			fmt.Println("not in leader role, ignore CommitCommitteeMessage")
			break
		}

		success := conR.csLeader.ProcessCommitMsg(msg, peer)
		if success == false {
			fmt.Println("process CommitCommitteeMessage failed")
		}

	case *ProposalBlockMessage:
		fmt.Println("receives ProposalBlockMessage ...")
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			fmt.Println("not in validator role, ignore ProposalBlockMessage")
			break
		}

		success := conR.csValidator.ProcessProposalBlockMessage(msg, peer)
		if success == false {
			fmt.Println("process ProposalBlockMessage failed")
		}

	case *NotaryAnnounceMessage:
		fmt.Println("receives NotaryAnnounceMessage ...")
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			fmt.Println("not in validator role, ignore NotaryAnnounceMessage")
			break
		}

		success := conR.csValidator.ProcessNotaryAnnounceMessage(msg, peer)
		if success == false {
			fmt.Println("process NotaryAnnounceMessage failed")
		}

	case *NotaryBlockMessage:
		fmt.Println("receives NotaryBlockMessage ...")
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			fmt.Println("not in validator role, ignore NotaryBlockMessage")
			break
		}

		success := conR.csValidator.ProcessNotaryBlockMessage(msg, peer)
		if success == false {
			fmt.Println("process NotaryBlockMessage failed")
		}

	case *VoteForProposalMessage:
		fmt.Println("receives VoteForProposalMessage ...")
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_PROPOSER) == 0 ||
			(conR.csProposer == nil) {
			fmt.Println("not in proposer role, ignore VoteForProposalMessage")
			break
		}

		success := conR.csProposer.ProcessVoteForProposal(msg, peer)
		if success == false {
			fmt.Println("process VoteForProposal failed")
		}

	case *VoteForNotaryMessage:
		fmt.Println("receives VoteForNotaryMessage ...")
		ch := msg.CSMsgCommonHeader

		if ch.MsgSubType == VOTE_FOR_NOTARY_ANNOUNCE {
			// vote for notary announce
			if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_LEADER) == 0 ||
				(conR.csLeader == nil) {
				fmt.Println("not in leader role, ignore VoteForNotaryMessage")
				break
			}

			success := conR.csLeader.ProcessVoteNotaryAnnounce(msg, peer)
			if success == false {
				fmt.Println("process VoteForNotary(Announce) failed")
			}

		} else if ch.MsgSubType == VOTE_FOR_NOTARY_BLOCK {
			if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_PROPOSER) == 0 ||
				(conR.csProposer == nil) {
				fmt.Println("not in proposer role, ignore VoteForNotaryMessage")
				break
			}

			success := conR.csProposer.ProcessVoteForNotary(msg, peer)
			if success == false {
				fmt.Println("process VoteForNotary(Block) failed")
			}
		} else {
			fmt.Println("Unknown MsgSubType", ch.MsgSubType)
		}
	case *MoveNewRoundMessage:
		fmt.Println("receives MoveNewRoundMessage ...")
		if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
			(conR.csValidator == nil) {
			fmt.Println("not in validator role, ignore MoveNewRoundMessage")
			break
		}

		success := conR.csValidator.ProcessMoveNewRoundMessage(msg, peer)
		if success == false {
			fmt.Println("process MoveNewRound failed")
		}

	default:
		fmt.Println("Unknown msg type", reflect.TypeOf(msg))
	}
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

	for {
		var mi consensusMsgInfo
		select {
		case mi = <-conR.peerMsgQueue:
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			fmt.Println("received msg from peerMsgQueue ...")
			conR.handleMsg(mi)
		case mi = <-conR.internalMsgQueue:
			// handles proposals, block parts, votes
			fmt.Println("received msg from InternalMsgQueue ...")
			conR.handleMsg(mi)
		case ti := <-conR.schedulerQueue:
			conR.HandleSchedule(ti)

			//case ki := <-conR.KBlockDataQueue:
			//conR.HandleKBlockData(ki)

			/*******
			case pi := <-conR.packerInfoQueue:
				conR.HandlePackerInfo(pi)
			case <-conR.Quit():
				onExit(conR)
			************/
		}
	}
}

func (conR *ConsensusReactor) receivePeerMsg(w http.ResponseWriter, r *http.Request) {
	var base = 10
	var size = 16
	defer r.Body.Close()
	var params map[string]string
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		fmt.Errorf("%v\n", err)
		respondWithJson(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	peerIP := net.ParseIP(params["peer_ip"])
	//peerID := p2p.ID(params["peer_id"])
	peerPort, convErr := strconv.ParseUint(params["port"], base, size)
	if convErr != nil {
		fmt.Errorf("Failed to convert to uint.")
	}
	peerPortUint16 := uint16(peerPort)
	peerAddr := types.NetAddress{
		//ID:   peerID,
		IP:   peerIP,
		Port: peerPortUint16,
	}
	p := ConsensusPeer{netAddr: peerAddr}
	msgByteSlice, _ := hex.DecodeString(params["message"])
	mi := consensusMsgInfo{
		Msg:    msgByteSlice,
		csPeer: &p,
	}
	conR.peerMsgQueue <- mi
	respondWithJson(w, http.StatusOK, map[string]string{"result": "success"})
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (conR *ConsensusReactor) receivePeerMsgRoutine() {

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},    // All origins
		AllowedMethods: []string{"POST"}, // Only allows POST requests
	})
	r := mux.NewRouter()
	r.HandleFunc("/peer", conR.receivePeerMsg).Methods("POST")
	if err := http.ListenAndServe(":8080", c.Handler(r)); err != nil {
		fmt.Errorf("HTTP receiver error!")
	}
}

//Entry point of new consensus
func (conR *ConsensusReactor) NewConsensusStart() int {
	fmt.Println("    Starting New Consensus ...")

	/***** Yang: Common init is based on role: leader and normal validator.
	 ***** Leader generate bls type/params/system and send out those params
	 ***** by announce message. Validators receives announce and do common init

	// initialize consensus common, Common is calling the C libary,
	// need to deinit to avoid the memory leak
	conR.csCommon = NewConsensusCommon(conR)
	******/

	// Uncomment following to enable peer messages between nodes
	go conR.receivePeerMsgRoutine()

	// Start receive routine
	go conR.receiveRoutine() //only handles from channel
	return 0
}

// called by reactor stop
func (conR *ConsensusReactor) NewConsensusStop() int {
	fmt.Println("Stop New Consensus ...")

	// Deinitialize consensus common
	conR.csCommon.ConsensusCommonDeinit()

	return 0
}

// -------
// Enter validator
func (conR *ConsensusReactor) enterConsensusValidator() int {
	fmt.Println("enter consensus validator")

	conR.csValidator = NewConsensusValidator(conR)
	conR.csRoleInitialized |= CONSENSUS_COMMIT_ROLE_VALIDATOR

	return 0
}

func (conR *ConsensusReactor) exitConsensusValidator() int {

	fmt.Println("exit consensus validator")
	conR.csValidator = nil
	conR.csRoleInitialized &= ^CONSENSUS_COMMIT_ROLE_VALIDATOR

	return 0
}

// Enter proposer
func (conR *ConsensusReactor) enterConsensusProposer() int {
	fmt.Println("enter consensus proposer")

	conR.csProposer = NewCommitteeProposer(conR)
	conR.csRoleInitialized |= CONSENSUS_COMMIT_ROLE_PROPOSER

	return 0
}

func (conR *ConsensusReactor) exitConsensusProposer() int {
	fmt.Println("enter consensus proposer")

	conR.csProposer = nil
	conR.csRoleInitialized &= ^CONSENSUS_COMMIT_ROLE_PROPOSER

	return 0
}

// Enter leader
func (conR *ConsensusReactor) enterConsensusLeader() int {
	fmt.Println("enter consensus leader")

	// init consensus common as leader
	// need to deinit to avoid the memory leak
	conR.csCommon = NewConsensusCommon(conR)

	conR.csLeader = NewCommitteeLeader(conR)
	conR.csRoleInitialized |= CONSENSUS_COMMIT_ROLE_LEADER

	return 0
}

func (conR *ConsensusReactor) exitConsensusLeader() int {
	fmt.Println("exit consensus leader")

	conR.csLeader = nil
	conR.csRoleInitialized &= ^CONSENSUS_COMMIT_ROLE_LEADER

	return 0
}

// XXX. For test only
func (conR *ConsensusReactor) sendConsensusMsg(msg *ConsensusMessage, csPeer *ConsensusPeer) bool {
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		fmt.Errorf("Msg exceeds max size (%d > %d)", len(rawMsg), maxMsgSize)
		return false
	}

	fmt.Println("try send msg out, size: ", len(rawMsg))
	fmt.Println(hex.Dump(rawMsg))

	if csPeer == nil {
		conR.internalMsgQueue <- consensusMsgInfo{rawMsg, nil}
	} else {
		//conR.peerMsgQueue <- consensusMsgInfo{rawMsg, csPeer}
		/*************
		payload := map[string]interface{}{
			"message":   hex.EncodeToString(rawMsg),
			"peer_ip":   csPeer.netAddr.IP.String(),
			"peer_id":   string(csPeer.netAddr.ID),
			"peer_port": string(csPeer.netAddr.Port),
		}
		**************/
		myNetAddr := conR.curCommittee.Validators[conR.curCommitteeIndex].NetAddr
		payload := map[string]interface{}{
			"message": hex.EncodeToString(rawMsg),
			"peer_ip": myNetAddr.IP.String(),
			//"peer_id":   string(myNetAddr.ID),
			"peer_port": string(myNetAddr.Port),
		}

		jsonStr, err := json.Marshal(payload)
		if err != nil {
			fmt.Errorf("Failed to marshal message dict to json string")
			return false
		}

		resp, err := http.Post("http://"+csPeer.netAddr.IP.String()+":8080/peer", "application/json", bytes.NewBuffer(jsonStr))
		if err != nil {
			fmt.Println("Failed to send message to peer: ", csPeer.netAddr.IP.String())
			fmt.Println(err)
			return false
		}
		fmt.Println("sent message to peer: ", csPeer.netAddr.IP.String())
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
	}
	return true
}

//----------------------------------------------------------------------------
// each node create signing message based on current information and sign part
// of them.

const (
	MSG_SIGN_OFFSET_DEFAULT = uint(0)
	MSG_SIGN_LENGTH_DEFAULT = uint(110)
)

// Sign Announce Committee
// "Announce Committee Message: Leader <pubkey 64(hexdump 32x2) bytes> CommitteeID <8 (4x2)bytes> Height <16 (8x2) bytes> Round <8(4x2)bytes>
func (conR *ConsensusReactor) BuildAnnounceSignMsg(pubKey ecdsa.PublicKey, committeeID uint32, height uint64, round uint32) string {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, committeeID)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	r := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(r, round)
	return fmt.Sprintf("%s %s %s %s %s %s %s %s", "Announce Committee Message: Leader", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"CommitteeID", hex.EncodeToString(c), "Height", hex.EncodeToString(h),
		"Round", hex.EncodeToString(r))
}

// Sign Propopal Message
// "Proposal Block Message: Proposer <pubkey 64(32x3)> BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func (conR *ConsensusReactor) BuildProposalBlockSignMsg(pubKey ecdsa.PublicKey, blockType uint32, height uint64, round uint32) string {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, blockType)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	r := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(r, round)
	return fmt.Sprintf("%s %s %s %s %s %s %s %s", "Proposal Block Message: Proposer", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"BlockType", hex.EncodeToString(c), "Height", hex.EncodeToString(h),
		"Round", hex.EncodeToString(r))
}

// Sign Notary Announce Message
// "Announce Notarization Message: Leader <pubkey 64(32x3)> CommitteeID <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func (conR *ConsensusReactor) BuildNotaryAnnounceSignMsg(pubKey ecdsa.PublicKey, committeeID uint32, height uint64, round uint32) string {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, committeeID)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	r := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(r, round)
	return fmt.Sprintf("%s %s %s %s %s %s %s %s", "Announce Notarization Message: Leader", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"CommitteeID", hex.EncodeToString(c), "Height", hex.EncodeToString(h),
		"Round", hex.EncodeToString(r))
}

// Sign Notary Block Message
// "Block Notarization Message: Proposer <pubkey 64(32x3)> BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func (conR *ConsensusReactor) BuildNotaryBlockSignMsg(pubKey ecdsa.PublicKey, blockType uint32, height uint64, round uint32) string {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, blockType)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	r := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(r, round)
	return fmt.Sprintf("%s %s %s %s %s %s %s %s", "Proposal Block Message: Proposer", hex.EncodeToString(crypto.FromECDSAPub(&pubKey)),
		"BlockType", hex.EncodeToString(c), "Height", hex.EncodeToString(h),
		"Round", hex.EncodeToString(r))
}

//======end of New consensus =========================================
//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------

// Consensus Topology Peer
type ConsensusPeer struct {
	netAddr types.NetAddress
}

func newConsensusPeer(ip net.IP, port uint16) *ConsensusPeer {
	return &ConsensusPeer{
		netAddr: types.NetAddress{
			IP:   ip,
			Port: port,
		},
	}
}

// XXX. Zilliqa just use socket, can we use http to simplify this
func (cp *ConsensusPeer) sendConsensusMsg(msg *ConsensusMessage) bool {
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		fmt.Errorf("Msg exceeds max size (%d > %d)", len(rawMsg), maxMsgSize)
		return false
	}

	// XXX: need to send rawMsg to peer
	fmt.Println("try send msg out, size: ", len(rawMsg))
	fmt.Println(hex.Dump(rawMsg))

	return true
}

//-----------------------------------------------------------------------------
// Messages

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface{}

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)

	// New consensus
	cdc.RegisterConcrete(&AnnounceCommitteeMessage{}, "tendermint/AnnounceCommittee", nil)
	cdc.RegisterConcrete(&CommitCommitteeMessage{}, "tendermint/CommitCommittee", nil)
	cdc.RegisterConcrete(&ProposalBlockMessage{}, "tendermint/ProposalBlock", nil)
	cdc.RegisterConcrete(&NotaryAnnounceMessage{}, "tendermint/NotaryAnnounce", nil)
	cdc.RegisterConcrete(&NotaryBlockMessage{}, "tendermint/NotaryBlock", nil)
	cdc.RegisterConcrete(&VoteForProposalMessage{}, "tendermint/VoteForProposal", nil)
	cdc.RegisterConcrete(&VoteForNotaryMessage{}, "tendermint/VoteForNotary", nil)
	cdc.RegisterConcrete(&MoveNewRoundMessage{}, "tendermint/MoveNewRound", nil)
}

func decodeMsg(bz []byte) (msg ConsensusMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------
// new consensus
// ConsensusMsgCommonHeader
type ConsensusMsgCommonHeader struct {
	Height     int64
	Round      int
	Sender     []byte //ecdsa.PublicKey
	Timestamp  time.Time
	MsgType    byte
	MsgSubType byte
}

// New Consensus
// Message Definitions
//---------------------------------------
// AnnounceCommitteeMessage is sent when new committee is relayed. The leader of new committee
// send out to announce the new committee is setup.
type AnnounceCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	AnnouncerID   []byte //ecdsa.PublicKey
	CommitteeID   uint32
	CommitteeSize int
	Nonce         uint64 //nonce is 8 bytes

	CSParams       []byte
	CSSystem       []byte
	CSLeaderPubKey []byte //bls.PublicKey
	KBlockHeight   int64
	POWBlockHeight int64

	SignOffset uint
	SignLength uint
	//possible POW info
	//...
}

// String returns a string representation.
func (m *AnnounceCommitteeMessage) String() string {
	return fmt.Sprintf("[AnnounceCommittee H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

// CommitCommitteMessage is sent after announce committee is received. Told the Leader
// there is enough member to setup the committee.
type CommitCommitteeMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	CommitteeID   uint32
	CommitteeSize int
	CommitterID   []byte //ecdsa.PublicKey

	CSCommitterPubKey  []byte //bls.PublicKey
	CommitterSignature []byte //bls.Signature
	CommitterIndex     int
	SignedMessageHash  [32]byte
}

// String returns a string representation.
func (m *CommitCommitteeMessage) String() string {
	return fmt.Sprintf("[CommitCommittee H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//-------------------------------------

// ProposalBlockMessage is sent when a new mblock is proposed.
type ProposalBlockMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	CommitteeID      uint32
	ProposerID       []byte //ecdsa.PublicKey
	CSProposerPubKey []byte //bls.PublicKey
	KBlockHeight     int64
	SignOffset       uint
	SignLength       uint
	ProposedSize     int
	ProposedBlock    []byte
}

// String returns a string representation.
func (m *ProposalBlockMessage) String() string {
	return fmt.Sprintf("[ProposalBlockMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//-------------------------------------
// NotaryBlockMessage is sent when a prevois proposal reaches 2/3
type NotaryAnnounceMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	AnnouncerID   []byte //ecdsa.PublicKey
	CommitteeID   uint32
	CommitteeSize int

	SignOffset             uint
	SignLength             uint
	VoterBitArray          cmn.BitArray
	VoterAggSignature      []byte //bls.Signature
	CommitteeActualSize    int
	CommitteeActualMembers []block.CommitteeInfo
}

// String returns a string representation.
func (m *NotaryAnnounceMessage) String() string {
	return fmt.Sprintf("[NotaryAnnounceMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//-------------------------------------
// NotaryBlockMessage is sent when a prevois proposal reaches 2/3
type NotaryBlockMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	ProposerID        []byte //ecdsa.PublicKey
	CommitteeID       uint32
	CommitteeSize     int
	SignOffset        uint
	SignLength        uint
	VoterBitArray     cmn.BitArray
	VoterAggSignature []byte //bls.Signature
}

// String returns a string representation.
func (m *NotaryBlockMessage) String() string {
	return fmt.Sprintf("[NotaryBlockMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//-------------------------------------

// VoteResponseMessage is sent when voting for a proposal (or lack thereof).
type VoteForProposalMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	VoterID           []byte //ecdsa.PublicKey
	VoteSummary       int64
	CSVoterPubKey     []byte //bls.PublicKey
	VoterSignature    []byte //bls.Signature
	VoterIndex        int
	SignedMessageHash [32]byte
}

// String returns a string representation.
func (m *VoteForProposalMessage) String() string {
	return fmt.Sprintf("[VoteForProposalMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//------------------------------------
// VoteResponseMessage is sent when voting for a proposal (or lack thereof).
type VoteForNotaryMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader //subtype: 1 - vote for Announce 2 - vote for proposal

	VoterID           []byte //ecdsa.PublicKey
	VoteSummary       int64
	CSVoterPubKey     []byte //bls.PublicKey
	VoterSignature    []byte //bls.Signature
	VoterIndex        int
	SignedMessageHash [32]byte
}

// String returns a string representation.
func (m *VoteForNotaryMessage) String() string {
	return fmt.Sprintf("[VoteForNotaryMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

//------------------------------------
// MoveNewRound message:
// 1. when a proposer can not get the consensus, so it sends out
// this message to give up.
// 2. Proposer disfunctional, the next proposer send out it after a certain time.
//
type MoveNewRoundMessage struct {
	CSMsgCommonHeader ConsensusMsgCommonHeader

	Height      int64
	CurRound    int
	NewRound    int
	CurProposer []byte //ecdsa.PublicKey
	NewProposer []byte //ecdsa.PublicKey
}

// String returns a string representation.
func (m *MoveNewRoundMessage) String() string {
	return fmt.Sprintf("[MoveNewRoundMessage H:%v R:%v S:%v Type:%v]",
		m.CSMsgCommonHeader.Height, m.CSMsgCommonHeader.Round,
		m.CSMsgCommonHeader.Sender, m.CSMsgCommonHeader.MsgType)
}

// -------------------------------------------------------------------------
// New consensus timed schedule util
type Scheduler func(conR *ConsensusReactor) bool

type consensusTimeOutInfo struct {
	Duration time.Duration
	Height   int64 //Hight when triggered
	Round    int   //Round when triggered
	fn       Scheduler
	arg      *ConsensusReactor
}

func (ti *consensusTimeOutInfo) String() string {
	return fmt.Sprintf("%v ; %d/%d %v %v", ti.Duration, ti.Height, ti.Round, ti.fn, ti.arg)
}

//TBD: implemente timed schedule, Duration is not used right now
func (conR *ConsensusReactor) ScheduleLeader(d time.Duration) bool {
	ti := consensusTimeOutInfo{
		Duration: d,
		Height:   conR.curHeight,
		Round:    conR.curRound,
		fn:       HandleScheduleLeader,
		arg:      conR,
	}

	conR.schedulerQueue <- ti
	return true
}

func (conR *ConsensusReactor) ScheduleValidator(d time.Duration) bool {
	ti := consensusTimeOutInfo{
		Duration: d,
		Height:   conR.curHeight,
		Round:    conR.curRound,
		fn:       HandleScheduleValidator,
		arg:      conR,
	}

	conR.schedulerQueue <- ti
	return true
}

func (conR *ConsensusReactor) ScheduleProposer(d time.Duration) bool {
	ti := consensusTimeOutInfo{
		Duration: d,
		Height:   conR.curHeight,
		Round:    conR.curRound,
		fn:       HandleScheduleProposer,
		arg:      conR,
	}

	conR.schedulerQueue <- ti
	return true
}

// -------------------------------
func HandleScheduleLeader(conR *ConsensusReactor) bool {
	if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_LEADER) == 0 ||
		(conR.csLeader == nil) {
		conR.enterConsensusLeader()
	}
	conR.csLeader.GenerateAnnounceMsg()
	return true
}

func HandleScheduleProposer(conR *ConsensusReactor) bool {
	if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_PROPOSER) == 0 ||
		(conR.csProposer == nil) {
		conR.enterConsensusProposer()
	}
	conR.csProposer.ProposalBlockMsg(true)
	return true
}

func HandleScheduleValidator(conR *ConsensusReactor) bool {
	if (conR.csRoleInitialized&CONSENSUS_COMMIT_ROLE_VALIDATOR) == 0 ||
		(conR.csValidator == nil) {
		conR.enterConsensusValidator()
	}
	// Validator only responses the incoming message
	return true
}

// Handle Schedules from conR.scheduleQueue
func (conR *ConsensusReactor) HandleSchedule(ti consensusTimeOutInfo) bool {
	if ti.arg != conR {
		fmt.Println("ConsensusReactor changed ...")
		return false
	}
	fmt.Println("Handle schedule at height", ti.Height, "round", ti.Round, "scheduling", ti.fn)
	ti.fn(ti.arg)
	return true
}

//////////////////////////////////////////////////////
// Consensus module handle received nonce from kblock
func (conR *ConsensusReactor) ConsensusHandleReceivedNonce(kBlockHeight int64, nonce uint64) {
	fmt.Println("Consensus receives a nonce ...", nonce, "kBlockHeight", kBlockHeight)

	//XXX: Yang:
	//conR.lastKBlockHeight = kBlockHeight
	conR.curNonce = nonce

	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, nonce)
	role, inCommittee := conR.NewValidatorSetByNonce(buf)
	fmt.Println("receives nonce", nonce, "inCommittee", inCommittee, "role", role)

	if role == CONSENSUS_COMMIT_ROLE_LEADER {
		fmt.Println("I am committee leader for nonce", nonce)
		conR.ScheduleLeader(0)
	} else if role == CONSENSUS_COMMIT_ROLE_VALIDATOR {
		fmt.Println("I am committee validator for nonce", nonce)
		conR.ScheduleValidator(0)
	}

}

//-----------------------------------------------------------
//---------------block store new wrappers routines ----------
// XXX: Yang: moved to consensus_block.go

/*******************
type CommitteeInfo struct {
	PubKey      ecdsa.PublicKey // committee pubkey
	VotingPower int64
	Accum       int64
	NetAddr     types.NetAddress
	CSPubKey    []byte // Bls pubkey
	CSIndex     int    // Index, corresponding to the bitarray
}

func NewCommitteeInfo(pubKey ecdsa.PublicKey, power int64, accum int64, netAddr types.NetAddress, csPubKey []byte, csIndex int) *CommitteeInfo {
	return &CommitteeInfo{
		PubKey:      pubKey,
		VotingPower: power,
		Accum:       accum,
		NetAddr:     netAddr,
		CSPubKey:    csPubKey,
		CSIndex:     csIndex,
	}
}

func (conR *ConsensusReactor) finalizeCommitBlock(block *blockchain.NewBlock) bool {

	height := block.BlockHeight
	if (conR.curHeight + 1) != height {
		conR.Logger.Error(fmt.Sprintf("finalizeCommitBlock(%v): Invalid height. Current: %v/%v", height, conR.curHeight, conR.curRound))
		return false
	}

	//get block reactor
	bcR := blockchain.GetGlobBlockChainReactor()
	if bcR == nil {
		panic("Error getting block chain reactor ")
		return false
	}

	blockStore := bcR.GetBlockStore()
	fmt.Println("blockStore Height", blockStore.Height(), "commit height", height)
	if height <= blockStore.Height() {
		fmt.Println("Height mismatch. my height", height, "stored height", blockStore.Height())
		return false
	}

	// XXX: need to validate the block with newEvidence
	//if blockchain.ValidateBlock(block) == false {
	//	fmt.Println("Validate block failed ...")
	//	return false
	//}

	// set prevHash
	if height > 0 {
		prevBlock := blockStore.LoadBlock(height - 1)
		if prevBlock != nil {
			block.SetBlockPrevHash(prevBlock.GetHash())
		}
	}

	// get current block hash
	//blockHash := block.getHash()

	// save this block to persistence store
	blockParts := block.MakePartSet(types.BlockPartSizeBytes)
	blockStore.SaveBlock(block, blockParts)

	// block is saved. before broadcast out, update pool height to indicated I
	// already have this block.
	bcR.GetBlockPool().IncrPoolHeight()
	bcR.BroadcastBlock(block)

	// apply block???

	return true
}

//build block committee info part
func (conR *ConsensusReactor) BuildCommitteeInfoFromMember(cms []CommitteeMember) []CommitteeInfo {
	cis := []CommitteeInfo{}

	for _, cm := range cms {
		ci := NewCommitteeInfo(cm.PubKey, cm.VotingPower, cm.Accum, cm.NetAddr,
			conR.csCommon.system.PubKeyToBytes(cm.CSPubKey), cm.CSIndex)
		cis = append(cis, *ci)
	}
	return (cis)
}

//de-serialize the block committee info part
func (conR *ConsensusReactor) BuildCommitteeMemberFromInfo(cis []CommitteeInfo) []CommitteeMember {
	cms := []CommitteeMember{}
	for _, ci := range cis {
		cm := NewCommitteeMember()
		cm.PubKey = ci.PubKey
		cm.VotingPower = ci.VotingPower
		cm.NetAddr = ci.NetAddr

		CSPubKey, err := conR.csCommon.system.PubKeyFromBytes(ci.CSPubKey)
		if err != nil {
			panic(err)
		}
		cm.CSPubKey = CSPubKey
		cm.CSIndex = ci.CSIndex

		cms = append(cms, *cm)
	}
	return (cms)
}

//build block committee info part
func (conR *ConsensusReactor) MakeBlockCommitteeInfo(cms []CommitteeMember) []byte {
	cis := []CommitteeInfo{}

	for _, cm := range cms {
		ci := NewCommitteeInfo(cm.PubKey, cm.VotingPower, cm.Accum, cm.NetAddr,
			conR.csCommon.system.PubKeyToBytes(cm.CSPubKey), cm.CSIndex)
		cis = append(cis, *ci)
	}
	return (cdc.MustMarshalBinaryBare(&cis))
}

//de-serialize the block committee info part
func (conR *ConsensusReactor) DecodeBlockCommitteeInfo(ciBytes []byte) (cis []CommitteeInfo, err error) {
	err = cdc.UnmarshalBinaryBare(ciBytes, &cis)
	return
}
********************/
//============================================================================
//============================================================================
// Testing support code
//============================================================================
//============================================================================
type Delegate1 struct {
	Address     thor.Address     `json:"address"`
	PubKey      []byte           `json:"pub_key"`
	VotingPower int64            `json:"voting_power"`
	NetAddr     types.NetAddress `json:"network_addr"`

	Accum int64 `json:"accum"`
}

func configDelegates( /*myPubKey ecdsa.PublicKey*/ ) []*types.Delegate {
	delegates1 := make([]*Delegate1, 0)

	// Hack for compile
	file, err := ioutil.ReadFile("/home/yang/tree/src/github.com/dfinlab/thor-consensus/consensus/delegates.json" /*config.DefaultDelegatePath*/)
	if err != nil {
		fmt.Println("unable load delegate file", "error", err)
		fmt.Println("File is at", "$HOME" /*config.DefaultDelegatePath*/)
	}

	err = cdc.UnmarshalJSON(file, &delegates1)
	if err != nil {
		fmt.Println("Unable unmarshal delegate file")
		fmt.Println(err)
	}

	delegates := make([]*types.Delegate, 0)
	for i, d := range delegates1 {
		//fmt.Printf("Delegate %d:\n Address:%s\n Public Key: %v\nVoting Power:%d\n Network Address:%v\n Accum:%d\n",
		//	i+1, d.Address, d.PubKey, d.VotingPower, d.NetAddr, d.Accum)
		//fmt.Println()
		pubKey, err := crypto.UnmarshalPubkey(d.PubKey)
		if err != nil {
			fmt.Println("translate pubkey from bytes error")
			//privKey := crypto.ToECDSAUnsafe(d.PubKey)
			privKey, err1 := crypto.GenerateKey()
			if err1 != nil {
				fmt.Println("generate private key failed")
			}
			pubKey = &privKey.PublicKey
			pubKeyBytes := crypto.FromECDSAPub(pubKey)
			fmt.Println(hex.Dump(pubKeyBytes))
		}

		dd := types.NewDelegate(*pubKey, d.VotingPower)
		dd.Address = d.Address
		dd.NetAddr = d.NetAddr
		fmt.Printf("Delegate DD %d:\n Address:%s\n Public Key: %v\nVoting Power:%d\n Network Address:%v\n Accum:%d\n",
			i+1, dd.Address, dd.PubKey, dd.VotingPower, dd.NetAddr, d.Accum)

		delegates = append(delegates, dd)
	}
	return delegates
}
