package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/txpool"
)

const (
	MSG_KEEP_HEIGHT = 80
)

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
func (p *Pacemaker) AddressBlock(height uint64, round uint64) *pmBlock {
	if (p.proposalMap[height] != nil) && (p.proposalMap[height].Height == height) {
		//p.logger.Debug("Addressed block", "height", height, "round", round)
		return p.proposalMap[height]
	}

	p.logger.Info("Could not find out block", "height", height, "round", round)
	return nil
}

func (p *Pacemaker) receivePacemakerMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

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

	msg, msgHash, peer := mi.Msg, mi.MsgHash, mi.Peer
	typeName := getConcreteName(msg)
	msgHashHex := hex.EncodeToString(msgHash[:])[:MsgHashSize]

	// update msg cache to avoid duplicate
	existed := p.msgCache.Add(uint64(msg.Header().Height), msgHash)
	if existed {
		p.logger.Info("duplicate "+typeName+" , dropped ...", "peer", peer.name, "ip", peer.netAddr.IP.String(), "msgHash", msgHashHex)
		return
	}

	fromMyself := peer.netAddr.IP.String() == p.csReactor.GetMyNetAddr().IP.String()
	if fromMyself {
		p.logger.Info(fmt.Sprintf("Recv: %s", msg.String()), "peer", p.csReactor.GetMyName()+"(myself)", "ip", peer.netAddr.IP.String(), "msgHash", msgHashHex)
	} else {
		p.logger.Info(fmt.Sprintf("Recv: %s", msg.String()), "peer", peer.name, "ip", peer.netAddr.IP.String(), "msgHash", msgHashHex)
	}

	p.pacemakerMsgCh <- *mi

	// relay the message if these two conditions are met:
	// 1. the original message is not sent by myself
	// 2. it's a proposal message
	if fromMyself == false && typeName == "PMProposal" {
		height := msg.Header().Height
		round := msg.Header().Round
		peers, _ := p.GetRelayPeers(round)
		typeName := getConcreteName(mi.Msg)
		if len(peers) > 0 {
			p.logger.Info("Now, relay this "+typeName+"...", "height", height, "round", round, "msgHash", msgHashHex)
			p.asyncSendPacemakerMsg(mi.Msg, true, peers...)
		}
		p.msgCache.CleanTo(uint64(height - MSG_KEEP_HEIGHT))
	}
}

func (p *Pacemaker) GetRelayPeers(round int) ([]*ConsensusPeer, error) {
	peers := make([]*ConsensusPeer, 0)
	size := len(p.csReactor.curActualCommittee)
	myIndex := p.myActualCommitteeIndex
	if size == 0 {
		return make([]*ConsensusPeer, 0), errors.New("current actual committee is empty")
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
		member := p.csReactor.curActualCommittee[index]
		name := p.csReactor.GetCommitteeMemberNameByIP(member.NetAddr.IP)
		peers = append(peers, newConsensusPeer(name, member.NetAddr.IP, member.NetAddr.Port, p.csReactor.magic))
	}
	return peers, nil
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

		parent := p.proposalMap[b.Height-1]
		if parent.ProposedBlockType == KBlockType {
			p.logger.Info("the first stop committee block")
		} else if parent.ProposedBlockType == StopCommitteeType {
			grandParent := p.proposalMap[b.Height-2]
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
	blkID := blk.Header().ID()
	blkHeight := blk.Header().Number()
	state, err := p.csReactor.stateCreator.NewState(p.csReactor.chain.BestBlock().Header().StateRoot())
	if err != nil {
		p.logger.Error("revert state failed ...", "height", blkHeight, "id", blkID, "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

	now := uint64(time.Now().Unix())
	stage, receipts, err := p.csReactor.ProcessProposedBlock(parentHeader, blk, now)
	if err != nil {
		p.logger.Error("process block failed", "height", blkHeight, "id", blkID, "error", err)
		b.SuccessProcessed = false
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

func (p *Pacemaker) getProposerByRound(round int) *ConsensusPeer {
	proposer := p.csReactor.getRoundProposer(round)
	return newConsensusPeer(proposer.Name, proposer.NetAddr.IP, 8080, p.csReactor.magic)
}

// ------------------------------------------------------
// Message Delivery Utilities
// ------------------------------------------------------
func (p *Pacemaker) SendConsensusMessage(round uint64, msg ConsensusMessage, copyMyself bool) bool {
	myNetAddr := p.csReactor.GetMyNetAddr()
	myName := p.csReactor.GetMyName()
	myself := newConsensusPeer(myName, myNetAddr.IP, myNetAddr.Port, p.csReactor.magic)

	peers := make([]*ConsensusPeer, 0)
	switch msg.(type) {
	case *PMProposalMessage:
		peers, _ = p.GetRelayPeers(int(round))
	case *PMVoteMessage:
		proposer := p.getProposerByRound(int(round))
		peers = append(peers, proposer)
	case *PMNewViewMessage:
		nxtProposer := p.getProposerByRound(int(round))
		peers = append(peers, nxtProposer)
		myself = nil // don't send new view to myself
	}

	// send consensus message to myself first (except for PMNewViewMessage)
	if copyMyself && myself != nil {
		p.logger.Debug(fmt.Sprintf("Sending to myself: %v", msg.String()), "to", myName, "ip", myNetAddr.IP.String())
		p.asyncSendPacemakerMsg(msg, false, myself)
	}

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

	// broadcast consensus message to peers
	for _, peer := range peers {
		go func(peer *ConsensusPeer, data []byte, msgSummary string) {
			peer.sendPacemakerMsg(data, relay, msgSummary)
		}(peer, data, msgSummary)
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

		VoterSig: p.sigAggregator.sigBytes,
		VoterNum: p.sigAggregator.Count(),
	}, nil
}

func (p *Pacemaker) collectVoteSignature(voteMsg *PMVoteMessage) error {
	round := uint64(voteMsg.CSMsgCommonHeader.Round)
	if round == uint64(p.currentRound) && p.csReactor.amIRoundProproser(round) {
		// if round matches and I am proposer, collect signature and store in cache

		_, err := p.csReactor.csCommon.GetSystem().SigFromBytes(voteMsg.BlsSignature)
		if err != nil {
			return err
		}
		if voteMsg.VoterIndex < int64(p.csReactor.committeeSize) {
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

func (p *Pacemaker) verifyTimeoutCert(tc *PMTimeoutCert, height, round uint64) bool {
	if tc != nil {
		//FIXME: check timeout cert
		return tc.TimeoutHeight == height && tc.TimeoutRound <= round
	}
	return false
}

// for proposals which can not be addressed parent and QC node should
// put it to pending list and query the parent node
func (p *Pacemaker) sendQueryProposalMsg(queryHeight, queryRound, EpochID uint64, peer *ConsensusPeer) error {
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

	queryMsg, err := p.BuildQueryProposalMessage(queryHeight, queryRound, EpochID, myNetAddr)
	if err != nil {
		p.logger.Warn("failed to generate PMQueryProposal message", "err", err)
		return errors.New("failed to generate PMQueryProposal message")
	}
	p.asyncSendPacemakerMsg(queryMsg, false, peer)
	return nil
}

func (p *Pacemaker) pendingProposal(queryHeight, queryRound, epochID uint64, mi *consensusMsgInfo) error {
	if err := p.sendQueryProposalMsg(queryHeight, queryRound, epochID, mi.Peer); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(mi)
	return nil
}

// put it to pending list and query the parent node
func (p *Pacemaker) pendingNewView(queryHeight, queryRound, epochID uint64, mi *consensusMsgInfo) error {
	if err := p.sendQueryProposalMsg(queryHeight, queryRound, epochID, mi.Peer); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(mi)
	return nil
}

func (p *Pacemaker) checkPendingMessages(curHeight uint64) error {
	height := curHeight
	count := 0
	if pendingMsg, ok := p.pendingList.messages[height]; ok {
		count++
		p.pacemakerMsgCh <- pendingMsg
		// height++ //move higher
	}
	if count > 0 {
		p.logger.Info("Found pending messages", "from", height, "count", count)
	}

	lowest := p.pendingList.GetLowestHeight()
	if (height > lowest) && (height-lowest) >= 3*MSG_KEEP_HEIGHT {
		p.pendingList.CleanUpTo(height - MSG_KEEP_HEIGHT)
	}
	return nil
}
