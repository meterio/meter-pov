// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	sha256 "crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/meterio/meter-pov/meter"

	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
)

const (
	MSG_KEEP_HEIGHT = 80
)

type BlockProbe struct {
	Height uint32
	Round  uint32
	Type   uint32
	Raw    []byte
}

type PMProbeResult struct {
	Mode             string
	StartHeight      uint32
	StartRound       uint32
	CurRound         uint32
	MyCommitteeIndex int

	LastVotingHeight uint32
	LastOnBeatRound  uint32
	QCHigh           *block.QuorumCert
	BlockLeaf        *BlockProbe
	BlockExecuted    *BlockProbe
	BlockLocked      *BlockProbe

	ProposalCount int
	PendingCount  int
	PendingLowest uint32
}

// check a pmBlock is the extension of b_locked, max 10 hops
func (p *Pacemaker) IsExtendedFromBLocked(b *pmBlock) bool {

	i := int(0)
	tmp := b
	for i < 10 {
		if tmp == p.blockLocked {
			return true
		}
		if tmp = tmp.Parent; tmp == nil {
			break
		}
		i++
	}
	return false
}

// find out b b' b"
func (p *Pacemaker) AddressBlock(height uint32) *pmBlock {
	blk := p.proposalMap.Get(height)
	if blk != nil && blk.Height == height {
		//p.logger.Debug("Addressed block", "height", height, "round", round)
		return blk
	}

	p.logger.Debug("can not address block", "height", height)
	return nil
}

func (p *Pacemaker) receivePacemakerMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// handle no msg if pacemaker is stopped already
	if p.stopped {
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		p.logger.Error("Unrecognized payload", "err", err)
		return
	}
	mi, err := p.csReactor.UnmarshalMsg(data)
	if err != nil {
		p.logger.Error("Unmarshal error", "err", err)
		return
	}

	msg, sig, peer := mi.Msg, mi.Signature, mi.Peer
	typeName := getConcreteName(msg)
	peerName := peer.name
	peerIP := peer.netAddr.IP.String()
	existed := p.msgCache.Add(sig)
	if existed {
		p.logger.Debug("duplicate "+typeName+" , dropped ...", "peer", peerName, "ip", peerIP)
		return
	}

	if VerifyMsgType(msg) == false {
		p.logger.Error("invalid msg type, dropped ...", "peer", peerName, "ip", peerIP, "msg", msg.String())
		return
	}

	if VerifySignature(msg) == false {
		p.logger.Error("invalid signature, dropped ...", "peer", peerName, "ip", peerIP, "msg", msg.String())
		return
	}

	fromMyself := peer.netAddr.IP.String() == p.csReactor.GetMyNetAddr().IP.String()
	if fromMyself {
		peerName = peerName + "(myself)"
	}
	summary := msg.String()
	msgHashHex := mi.MsgHashHex()
	name := ""
	tail := ""
	split := strings.Split(summary, " ")
	if len(split) > 0 {
		name = split[0]
		tail = strings.Join(split[1:], " ")
	}
	p.logger.Info(fmt.Sprintf("Recv %s %s %s", name, msgHashHex, tail), "peer", peerName, "ip", peer.netAddr.IP.String(), "msgCh", fmt.Sprintf("%d/%d", len(p.pacemakerMsgCh), cap(p.pacemakerMsgCh)))

	if len(p.pacemakerMsgCh) < cap(p.pacemakerMsgCh) {
		p.pacemakerMsgCh <- *mi
	}

	// relay the message if these two conditions are met:
	// 1. the original message is not sent by myself
	// 2. it's a proposal message
	if fromMyself == false && typeName == "PMProposal" {
		p.relayMsg(*mi)
	}
}
func (p *Pacemaker) relayMsg(mi consensusMsgInfo) {
	msg := mi.Msg
	height := msg.Header().Height
	round := msg.Header().Round
	peers := p.GetRelayPeers(round)
	// typeName := getConcreteName(mi.Msg)
	msgHashHex := mi.MsgHashHex()
	if len(peers) > 0 {
		peerNames := make([]string, 0)
		for _, peer := range peers {
			peerNames = append(peerNames, peer.NameString())
		}
		msgSummary := (mi.Msg).String()
		p.logger.Info("Relay>> "+msgSummary, "to", strings.Join(peerNames, ", "), "height", height, "round", round, "msgHash", msgHashHex)
		// p.logger.Info("Now, relay this "+typeName+"...", "height", height, "round", round, "msgHash", mi.MsgHashHex())
		for _, peer := range peers {
			go peer.sendPacemakerMsg(mi.RawData, msgSummary, msgHashHex, true)
		}
		// p.asyncSendPacemakerMsg(mi.Msg, true, peers...)
	}

}

func (p *Pacemaker) GetRelayPeers(round uint32) []*ConsensusPeer {
	peers := make([]*ConsensusPeer, 0)
	size := len(p.csReactor.curActualCommittee)
	myIndex := p.csReactor.GetMyActualCommitteeIndex()
	if size == 0 {
		return make([]*ConsensusPeer, 0)
	}
	rr := int(round % uint32(size))
	if myIndex >= rr {
		myIndex = myIndex - rr
	} else {
		myIndex = myIndex + size - rr
	}

	indexes := GetRelayPeers(myIndex, size)
	for _, i := range indexes {
		index := i + rr
		if index >= size {
			index = index % size
		}
		member := p.csReactor.curActualCommittee[index]
		name := p.csReactor.GetDelegateNameByIP(member.NetAddr.IP)
		peers = append(peers, newConsensusPeer(name, member.NetAddr.IP, member.NetAddr.Port, p.csReactor.magic))
	}
	log.Debug("get relay peers result", "myIndex", myIndex, "committeeSize", size, "round", round, "indexes", indexes)
	return peers
}

func (p *Pacemaker) ValidateProposal(b *pmBlock) error {
	blockBytes := b.ProposedBlock
	blk, err := block.BlockDecodeFromBytes(blockBytes)
	if err != nil {
		p.logger.Error("Decode block failed", "err", err)
		return err
	}

	// special valiadte StopCommitteeType
	// possible 2 rounds of stop messagB
	if b.ProposedBlockType == StopCommitteeType {

		parent := p.proposalMap.Get(b.Height - 1)
		if parent.ProposedBlockType == KBlockType {
			p.logger.Info("the first stop committee block")
		} else if parent.ProposedBlockType == StopCommitteeType {
			grandParent := p.proposalMap.Get(b.Height - 2)
			if grandParent.ProposedBlockType == KBlockType {
				p.logger.Info("The second stop committee block")

			}
		}
	}

	/*
		if b.ProposedBlockInfo != nil {
			// if this proposal is proposed by myself, don't execute it again
			p.logger.Debug("this proposal is created by myself, skip the validation...")
			b.SuccessProcessed = true
			return nil
		}
	*/

	parentPMBlock := b.Parent
	if parentPMBlock == nil || parentPMBlock.ProposedBlock == nil {
		return errParentMissing
	}
	parentBlock, err := block.BlockDecodeFromBytes(parentPMBlock.ProposedBlock)
	if err != nil {
		return errDecodeParentFailed
	}
	parentHeader := parentBlock.Header()

	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		p.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}

	var txsInBlk []*tx.Transaction
	for _, tx := range blk.Transactions() {
		txsInBlk = append(txsInBlk, tx)
	}
	var txsToRemoved, txsToReturned func() bool
	if b.ProposedBlockType == KBlockType {
		txsToRemoved = func() bool { return true }
		txsToReturned = func() bool { return true }
	} else {
		txsToRemoved = func() bool {
			for _, tx := range txsInBlk {
				pool.Remove(tx.ID())
			}
			return true
		}
		txsToReturned = func() bool {
			for _, tx := range txsInBlk {
				pool.Add(tx)
			}
			return true
		}
	}

	//create checkPoint before validate block
	blkID := blk.ID()
	blkHeight := blk.Number()
	state, err := p.csReactor.stateCreator.NewState(p.csReactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "height", blkHeight, "id", blkID, "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

	now := uint64(time.Now().Unix())
	stage, receipts, err := p.csReactor.ProcessProposedBlock(parentHeader, blk, now)
	if err != nil && err != errKnownBlock {
		p.logger.Error("process block failed", "proposed", blk.Oneliner(), "err", err)
		b.SuccessProcessed = false
		b.ProcessError = err
		return err
	}

	b.ProposedBlockInfo = &ProposedBlockInfo{
		BlockType:     b.ProposedBlockType,
		ProposedBlock: blk,
		Stage:         stage,
		Receipts:      &receipts,
		CheckPoint:    checkPoint,
		txsToRemoved:  txsToRemoved,
		txsToReturned: txsToReturned,
	}

	b.SuccessProcessed = true

	p.logger.Info("Validated block proposal", "height", blkHeight, "id", blkID)
	return nil
}

func (p *Pacemaker) getProposerByRound(round uint32) *ConsensusPeer {
	proposer := p.csReactor.getRoundProposer(round)
	return newConsensusPeer(proposer.Name, proposer.NetAddr.IP, 8080, p.csReactor.magic)
}

// ------------------------------------------------------
// Message Delivery Utilities
// ------------------------------------------------------
func (p *Pacemaker) SendConsensusMessage(round uint32, msg ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.csReactor.GetMyNetAddr()
	myName := p.csReactor.GetMyName()
	myself := newConsensusPeer(myName, myNetAddr.IP, myNetAddr.Port, p.csReactor.magic)

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *PMProposalMessage:
		peers = p.GetRelayPeers(round)
	case *PMVoteMessage:
		proposer := p.getProposerByRound(round)
		peers = append(peers, proposer)
	case *PMNewViewMessage:
		nv := msg.(*PMNewViewMessage)
		if nv.Reason == HigherQCSeen {
			visited := make(map[string]bool)
			for i := 0; i < meter.NewViewPeersLimit; i++ {
				nxtProposer := p.getProposerByRound(round + uint32(i))
				if _, ok := visited[nxtProposer.String()]; !ok {
					peers = append(peers, nxtProposer)
					visited[nxtProposer.String()] = true
				}
			}
		} else {
			nxtProposer := p.getProposerByRound(round)
			peers = append(peers, nxtProposer)
		}
	}

	myselfInPeers := myself == nil
	for _, p := range peers {
		if p.netAddr.IP.String() == myNetAddr.IP.String() {
			myselfInPeers = true
			break
		}
	}
	// send consensus message to myself first (except for PMNewViewMessage)
	typeName := getConcreteName(msg)
	if copyMyself && !myselfInPeers {
		p.logger.Debug(fmt.Sprintf("Sending %v to myself", typeName))
		p.asyncSendPacemakerMsg(msg, false, myself)
	}

	peerNames := make([]string, 0)
	for _, p := range peers {
		peerNames = append(peerNames, p.name)
	}
	p.logger.Debug(fmt.Sprintf("Sending %v to peers: %v", typeName, strings.Join(peerNames, ",")))
	p.asyncSendPacemakerMsg(msg, false, peers...)
	return true
}

func (p *Pacemaker) asyncSendPacemakerMsg(msg ConsensusMessage, relay bool, peers ...*ConsensusPeer) bool {
	data, err := p.csReactor.MarshalMsg(&msg)
	if err != nil {
		fmt.Println("error marshaling message", err)
		return false
	}
	msgSummary := msg.String()
	msgHash := sha256.Sum256(data)
	msgHashHex := hex.EncodeToString(msgHash[:])[:8]

	peerNames := make([]string, 0)
	for _, peer := range peers {
		peerNames = append(peerNames, peer.NameString())
	}
	prefix := "Send>>"
	if relay {
		prefix = "Relay>>"
	}
	p.logger.Info(prefix+" "+msgSummary, "to", strings.Join(peerNames, ", "), "msgHash", msgHashHex)
	// broadcast consensus message to peers
	for _, peer := range peers {
		go peer.sendPacemakerMsg(data, msgSummary, msgHashHex, relay)
	}
	return true
}

func (p *Pacemaker) generateNewQCNode(b *pmBlock) (*pmQuorumCert, error) {
	aggSigBytes := p.sigAggregator.Aggregate()

	return &pmQuorumCert{
		QCNode: b,

		QC: &block.QuorumCert{
			QCHeight:         b.Height,
			QCRound:          b.Round,
			EpochID:          p.csReactor.curEpoch,
			VoterBitArrayStr: p.sigAggregator.BitArrayString(),
			VoterMsgHash:     p.sigAggregator.msgHash,
			VoterAggSig:      aggSigBytes,
			VoterViolation:   p.sigAggregator.violations,
		},

		// VoterSig: p.sigAggregator.sigBytes,
		// VoterNum: p.sigAggregator.Count(),
	}, nil
}

func (p *Pacemaker) collectVoteSignature(voteMsg *PMVoteMessage) error {
	round := voteMsg.CSMsgCommonHeader.Round
	if p.sigAggregator == nil {
		return errors.New("signature aggregator is nil, this proposal is not proposed by me")
	}
	if round == p.currentRound && p.csReactor.amIRoundProproser(round) {
		// if round matches and I am proposer, collect signature and store in cache

		_, err := p.csReactor.csCommon.GetSystem().SigFromBytes(voteMsg.BlsSignature)
		if err != nil {
			return err
		}
		if voteMsg.VoterIndex < p.csReactor.committeeSize {
			blsPubkey := p.csReactor.curCommittee.Validators[voteMsg.VoterIndex].BlsPubKey
			p.sigAggregator.Add(int(voteMsg.VoterIndex), voteMsg.SignedMessageHash, voteMsg.BlsSignature, blsPubkey)
			p.logger.Debug("Collected signature ", "index", voteMsg.VoterIndex, "signature", hex.EncodeToString(voteMsg.BlsSignature))
		} else {
			p.logger.Debug("Signature ignored because of msg hash mismatch")
		}
	} else {
		p.logger.Debug("Signature ignored because of round mismatch", "round", round, "currRound", p.currentRound)
	}
	// ignore the signatures if the round doesn't match
	return nil
}

func (p *Pacemaker) verifyTimeoutCert(tc *PMTimeoutCert, height, round uint32) bool {
	if tc != nil {
		//FIXME: check timeout cert
		valid := tc.TimeoutHeight <= height && tc.TimeoutRound == round
		if !valid {
			p.logger.Warn("Invalid Timeout Cert", "expected", fmt.Sprintf("H:%v,R:%v", tc.TimeoutHeight, tc.TimeoutRound), "proposal", fmt.Sprintf("H:%v,R:%v", height, round))
		}
		return valid
	}
	return false
}

// for proposals which can not be addressed parent and QC node should
// put it to pending list and query the parent node
func (p *Pacemaker) sendQueryProposalMsg(fromHeight, toHeight, queryRound uint32, EpochID uint64, peer *ConsensusPeer) error {
	// put this proposal to pending list, and sent out query
	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr

	// sometimes we find out addr is my self, protection is added here
	if myNetAddr.IP.Equal(peer.netAddr.IP) == true {
		for _, cm := range p.csReactor.curActualCommittee {
			if myNetAddr.IP.Equal(cm.NetAddr.IP) == false {
				p.logger.Warn("Query PMProposal with new node", "NetAddr", cm.NetAddr)
				peer = newConsensusPeer(cm.Name, cm.NetAddr.IP, cm.NetAddr.Port, p.csReactor.magic)
				break
			}
		}
	}

	// fromHeight must be less than or equal to toHeight, except when toHeight is 0
	// in this case, 0 is interpreted as infinity (or local qcHigh)
	if fromHeight > toHeight && toHeight != 0 {
		p.logger.Info("query not necessary", "fromHeight", fromHeight, "toHeight", toHeight)
		return nil
	}

	queryMsg, err := p.BuildQueryProposalMessage(fromHeight, toHeight, queryRound, EpochID, myNetAddr)
	if err != nil {
		p.logger.Warn("failed to generate PMQueryProposal message", "err", err)
		return errors.New("failed to generate PMQueryProposal message")
	}
	p.asyncSendPacemakerMsg(queryMsg, false, peer)
	return nil
}

func (p *Pacemaker) pendingProposal(queryHeight, queryRound uint32, epochID uint64, mi *consensusMsgInfo) error {
	fromHeight := p.lastVotingHeight
	if p.QCHigh != nil && p.QCHigh.QCNode != nil && fromHeight < p.QCHigh.QCNode.Height {
		fromHeight = p.QCHigh.QCNode.Height
	}
	if err := p.sendQueryProposalMsg(fromHeight, queryHeight, queryRound, epochID, mi.Peer); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(mi)
	return nil
}

// put it to pending list and query the parent node
func (p *Pacemaker) pendingNewView(queryHeight, queryRound uint32, epochID uint64, mi *consensusMsgInfo) error {
	bestQC := p.csReactor.chain.BestQC()
	fromHeight := p.lastVotingHeight
	if p.QCHigh != nil && p.QCHigh.QCNode != nil && fromHeight < p.QCHigh.QCNode.Height {
		fromHeight = bestQC.QCHeight
	}
	if err := p.sendQueryProposalMsg(fromHeight, queryHeight, queryRound, epochID, mi.Peer); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(mi)
	return nil
}

func (p *Pacemaker) checkPendingMessages(curHeight uint32) error {
	height := curHeight
	count := 0
	if pendingMsg, ok := p.pendingList.messages[height]; ok {
		count++
		// height++ //move higher
		capacity := cap(p.pacemakerMsgCh)
		msgs := make([]consensusMsgInfo, 0)
		for len(p.pacemakerMsgCh) > 0 {
			msgs = append(msgs, <-p.pacemakerMsgCh)
		}

		// promote pending msg to the very next
		p.pacemakerMsgCh <- pendingMsg

		for _, msg := range msgs {
			if len(p.pacemakerMsgCh) < capacity {
				p.pacemakerMsgCh <- msg
			}
		}
	}
	if count > 0 {
		p.logger.Info("Found pending message", "height", height)
	}

	lowest := p.pendingList.GetLowestHeight()
	if (height > lowest) && (height-lowest) >= 3*MSG_KEEP_HEIGHT {
		p.pendingList.CleanUpTo(height - MSG_KEEP_HEIGHT)
	}
	return nil
}

func (p *Pacemaker) Probe() *PMProbeResult {
	result := &PMProbeResult{
		Mode:             p.mode.String(),
		StartHeight:      p.startHeight,
		StartRound:       p.startRound,
		CurRound:         p.currentRound,
		MyCommitteeIndex: p.myActualCommitteeIndex,

		LastVotingHeight: p.lastVotingHeight,
		LastOnBeatRound:  p.lastOnBeatRound,
		QCHigh:           p.QCHigh.QC,
	}
	if p.QCHigh != nil && p.QCHigh.QC != nil {
		result.QCHigh = p.QCHigh.QC
	}
	if p.blockLeaf != nil {
		result.BlockLeaf = &BlockProbe{Height: p.blockLeaf.Height, Round: p.blockLeaf.Round, Type: uint32(p.blockLeaf.ProposedBlockType), Raw: p.blockLeaf.ProposedBlock}
	}
	if p.blockExecuted != nil {
		result.BlockExecuted = &BlockProbe{Height: p.blockExecuted.Height, Round: p.blockExecuted.Round, Type: uint32(p.blockExecuted.ProposedBlockType), Raw: p.blockExecuted.ProposedBlock}
	}
	if p.blockLocked != nil {
		result.BlockLocked = &BlockProbe{Height: p.blockLocked.Height, Round: p.blockLocked.Round, Type: uint32(p.blockLocked.ProposedBlockType), Raw: p.blockLocked.ProposedBlock}
	}
	if p.proposalMap != nil {
		result.ProposalCount = p.proposalMap.Len()
	}
	if p.pendingList != nil {
		result.PendingCount = p.pendingList.Len()
		result.PendingLowest = p.pendingList.GetLowestHeight()
	}
	return result

}
