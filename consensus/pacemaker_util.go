package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	crypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dfinlab/meter/block"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/types"
)

type receivedConsensusMessage struct {
	msg  ConsensusMessage
	from types.NetAddress
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
func (p *Pacemaker) AddressBlock(height uint64, round uint64) *pmBlock {
	if (p.proposalMap[height] != nil) && (p.proposalMap[height].Height == height) {
		//p.csReactor.logger.Debug("Addressed block", "height", height, "round", round)
		return p.proposalMap[height]
	}

	p.csReactor.logger.Info("Could not find out block", "height", height, "round", round)
	return nil
}

func (p *Pacemaker) receivePacemakerMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var params map[string]string
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		p.csReactor.logger.Error("decode received messsage failed", "error", err)
		respondWithJson(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	peerIP := net.ParseIP(params["peer_ip"])
	respondWithJson(w, http.StatusOK, map[string]string{"result": "success"})
	peerPort, err := strconv.ParseInt(params["peer_port"], 10, 16)
	if err != nil {
		peerPort = 0
	}

	msgByteSlice, _ := hex.DecodeString(params["message"])
	msg, err := decodeMsg(msgByteSlice)
	if err != nil {
		p.csReactor.logger.Error("message decode error", "err", err)
		panic("message decode error")
	} else {
		typeName := getConcreteName(msg)
		var fromMyself bool
		if peerIP.String() == p.csReactor.GetMyNetAddr().IP.String() {
			p.logger.Info("Received pacemaker msg from myself", "type", typeName, "from", peerIP.String())
			fromMyself = true
		} else {
			p.logger.Info("Received pacemaker msg from peer", "msg", msg.String(), "from", peerIP.String())
			fromMyself = false
		}

		// check replay first, also include the proposal myself
		var height uint64
		var round int
		if typeName == "PMProposalMessage" {
			proposal := msg.(*PMProposalMessage)
			height = uint64(proposal.CSMsgCommonHeader.Height)
			round = proposal.CSMsgCommonHeader.Round
			if p.msgRelayInfo.InMap(&msgByteSlice, height, round) == true {
				p.logger.Info("received PMProposal, duplicated, dropped", "height", height, "round", round)
				return
			} else {
				p.logger.Info("received PMProposal, added", "height", height, "round", round)
				p.msgRelayInfo.Add(&msgByteSlice, height, round)
			}
		}

		from := types.NetAddress{IP: peerIP, Port: uint16(peerPort)}
		p.pacemakerMsgCh <- receivedConsensusMessage{msg, from}

		// take the action in the end
		// now relay this message if proposal
		if fromMyself == false && typeName == "PMProposalMessage" {
			peers, _ := p.GetRelayPeers(round)
			for _, peer := range peers {
				p.logger.Info("now, relay this proposal...", "peer", peer.String(), "height", height, "round", round)
				if peer.netAddr.IP.String() == p.csReactor.GetMyNetAddr().IP.String() {
					p.logger.Info("relay to myself, ignore ...")
					continue
				}
				go peer.sendData(from, typeName, msgByteSlice)
			}

			lowest := p.msgRelayInfo.GetLowestHeight()
			if (height > lowest) && (height-lowest) >= 3*RELAY_MSG_KEEP_HEIGHT {
				go p.msgRelayInfo.CleanUpTo(height - RELAY_MSG_KEEP_HEIGHT)
			}
		}
	}
}

func (p *Pacemaker) GetRelayPeers(round int) ([]*ConsensusPeer, error) {
	peers := []*ConsensusPeer{}
	size := len(p.csReactor.curActualCommittee)
	myIndex := p.myActualCommitteeIndex
	rr := round % size
	if myIndex >= rr {
		myIndex = myIndex - rr
	} else {
		myIndex = myIndex + size - rr
	}

	indexes := getRelayPeers(myIndex, size-1)
	for _, i := range indexes {
		index := i + rr
		if index >= size {
			index = index % size
		}
		member := p.csReactor.curActualCommittee[index]
		peers = append(peers, newConsensusPeer(member.NetAddr.IP, member.NetAddr.Port))
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
	p.logger.Info("Validate proposal", "type", b.ProposedBlockType, "block", blk.Oneliner())

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

	if b.ProposedBlockInfo != nil {
		// if this proposal is proposed by myself, don't execute it again
		p.logger.Debug("this proposal is created by myself, skip the validation...")
		b.SuccessProcessed = true
		return nil
	}

	parentPMBlock := b.Parent
	if parentPMBlock == nil || parentPMBlock.ProposedBlock == nil {
		return errParentMissing
	}
	parentBlock, err := block.BlockDecodeFromBytes(parentPMBlock.ProposedBlock)
	if err != nil {
		return errDecodeParentFailed
	}
	parentHeader := parentBlock.Header()

	now := uint64(time.Now().Unix())
	stage, receipts, err := p.csReactor.ProcessProposedBlock(parentHeader, blk, now)
	if err != nil {
		p.logger.Error("process block failed", "error", err)
		b.SuccessProcessed = false
		return err
	}

	b.ProposedBlockInfo = &ProposedBlockInfo{
		BlockType:     b.ProposedBlockType,
		ProposedBlock: blk,
		Stage:         stage,
		Receipts:      &receipts,
		txsToRemoved:  func() bool { return true },
	}

	b.SuccessProcessed = true

	p.logger.Info("Validated block")
	return nil
}

func (p *Pacemaker) isMine(key []byte) bool {
	myKey := crypto.FromECDSAPub(&p.csReactor.myPubKey)
	return bytes.Equal(key, myKey)
}

func (p *Pacemaker) getProposerByRound(round int) *ConsensusPeer {
	proposer := p.csReactor.getRoundProposer(round)
	return newConsensusPeer(proposer.NetAddr.IP, 8080)
}

func (p *Pacemaker) getConsensusPeerByPubkey(pubKey []byte) *ConsensusPeer {
	if cm := p.csReactor.GetCommitteeMember(pubKey); cm != nil {
		return newConsensusPeer(cm.NetAddr.IP, cm.NetAddr.Port)
	} else {
		return nil
	}
}

// ------------------------------------------------------
// Message Delivery Utilities
// ------------------------------------------------------
func (p *Pacemaker) SendConsensusMessage(round uint64, msg ConsensusMessage, copyMyself bool) bool {
	typeName := getConcreteName(msg)
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		p.logger.Error("Msg exceeds max size", "rawMsg=", len(rawMsg), "maxMsgSize=", maxMsgSize)
		return false
	}

	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr
	myself := newConsensusPeer(myNetAddr.IP, myNetAddr.Port)

	var peers []*ConsensusPeer
	switch msg.(type) {
	case *PMProposalMessage:
		peers, _ = p.GetRelayPeers(int(round))
	case *PMVoteForProposalMessage:
		proposer := p.getProposerByRound(int(round))
		peers = []*ConsensusPeer{proposer}
	case *PMNewViewMessage:
		nxtProposer := p.getProposerByRound(int(round))
		peers = []*ConsensusPeer{nxtProposer}
		myself = nil // don't send new view to myself
	}

	// send consensus message to myself first (except for PMNewViewMessage)
	if copyMyself && myself != nil {
		p.logger.Debug("Sending pacemaker msg to myself", "type", typeName, "to", myNetAddr.IP.String())
		myself.sendData(myNetAddr, typeName, rawMsg)
	}

	// broadcast consensus message to peers
	p.goes.Go(func() {
		for _, peer := range peers {
			hint := "Sending pacemaker msg to peer"
			if peer.netAddr.IP.String() == myNetAddr.IP.String() {
				hint = "Sending pacemaker msg to myself"
			}
			p.logger.Debug(hint, "type", typeName, "to", peer.netAddr.IP.String())
			peer.sendData(myNetAddr, typeName, rawMsg)
		}
	})
	return true
}

func (p *Pacemaker) SendMessageToPeers(msg ConsensusMessage, peers []*ConsensusPeer) bool {
	typeName := getConcreteName(msg)
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		p.logger.Error("Msg exceeds max size", "rawMsg=", len(rawMsg), "maxMsgSize=", maxMsgSize)
		return false
	}

	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr
	// broadcast consensus message to peers
	p.goes.Go(func() {
		for _, peer := range peers {
			hint := "Sending pacemaker msg to peer"
			if peer.netAddr.IP.String() == myNetAddr.IP.String() {
				hint = "Sending pacemaker msg to myself"
			}
			p.logger.Debug(hint, "type", typeName, "to", peer.netAddr.IP.String())
			peer.sendData(myNetAddr, typeName, rawMsg)
		}
	})
	return true
}

func (p *Pacemaker) generateNewQCNode(b *pmBlock) (*pmQuorumCert, error) {
	sigs := make([]bls.Signature, 0)
	msgHashes := make([][32]byte, 0)
	sigBytes := make([][]byte, 0)
	for _, s := range p.voteSigs {
		sigs = append(sigs, s.signature)
		sigBytes = append(sigBytes, p.csReactor.csCommon.system.SigToBytes(s.signature))
		msgHashes = append(msgHashes, s.msgHash)
	}
	aggSig := p.csReactor.csCommon.AggregateSign(sigs)
	aggSigBytes := p.csReactor.csCommon.system.SigToBytes(aggSig)

	voterBitArrayStr, _ := p.voterBitArray.MarshalJSON()
	return &pmQuorumCert{
		QCNode: b,

		QC: &block.QuorumCert{
			QCHeight:         b.Height,
			QCRound:          b.Round,
			EpochID:          p.csReactor.curEpoch,
			VoterBitArrayStr: string(voterBitArrayStr),
			VoterMsgHash:     msgHashes,
			VoterAggSig:      aggSigBytes,
		},

		VoterSig: sigBytes,
		VoterNum: uint32(len(p.voteSigs)),
	}, nil
}

func (p *Pacemaker) collectVoteSignature(voteMsg *PMVoteForProposalMessage) error {
	round := uint64(voteMsg.CSMsgCommonHeader.Round)
	if round == uint64(p.currentRound) && p.csReactor.amIRoundProproser(round) {
		// if round matches and I am proposer, collect signature and store in cache
		sigBytes, err := p.csReactor.csCommon.system.SigFromBytes(voteMsg.VoterSignature)
		if err != nil {
			return err
		}
		sig := &PMSignature{
			index:     voteMsg.VoterIndex,
			msgHash:   voteMsg.SignedMessageHash,
			signature: sigBytes,
		}
		p.voterBitArray.SetIndex(int(voteMsg.VoterIndex), true)
		p.voteSigs = append(p.voteSigs, sig)
		p.logger.Debug("Collected signature ", "index", voteMsg.VoterIndex, "signature", hex.EncodeToString(voteMsg.VoterSignature))
	} else {
		p.logger.Debug("Signature ignored because of round mismatch", "round", round, "currRound", p.currentRound)
	}
	// ignore the signatures if the round doesn't match
	return nil
}

func (p *Pacemaker) verifyTimeoutCert(tc *PMTimeoutCert, height, round uint64) bool {
	if tc != nil {
		//FIXME: check timeout cert
		return tc.TimeoutHeight == height && tc.TimeoutRound < round
	}
	return false
}

// for proposals which can not be addressed parent and QC node should
// put it to pending list and query the parent node
func (p *Pacemaker) sendQueryProposalMsg(queryHeight, queryRound, EpochID uint64, addr types.NetAddress) error {
	// put this proposal to pending list, and sent out query
	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr

	// sometimes we find out addr is my self, protection is added here
	if (myNetAddr.IP.Equal(addr.IP) == true) && (addr.Port == myNetAddr.Port) {
		for _, cm := range p.csReactor.curActualCommittee {
			if myNetAddr.IP.Equal(cm.NetAddr.IP) == false {
				p.logger.Warn("Query PMProposal with new node", "NetAddr", cm.NetAddr)
				addr = cm.NetAddr
				break
			}
		}
	}
	peers := []*ConsensusPeer{newConsensusPeer(addr.IP, addr.Port)}

	queryMsg, err := p.BuildQueryProposalMessage(queryHeight, queryRound, EpochID, myNetAddr)
	if err != nil {
		p.logger.Warn("failed to generate PMQueryProposal message", "err", err)
		return errors.New("failed to generate PMQueryProposal message")
	}
	p.SendMessageToPeers(queryMsg, peers)
	return nil
}

func (p *Pacemaker) pendingProposal(queryHeight, queryRound uint64, proposalMsg *PMProposalMessage, addr types.NetAddress) error {
	epochID := proposalMsg.CSMsgCommonHeader.EpochID
	if err := p.sendQueryProposalMsg(queryHeight, queryRound, epochID, addr); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(proposalMsg, addr)
	return nil
}

// put it to pending list and query the parent node
func (p *Pacemaker) pendingNewView(queryHeight, queryRound uint64, newViewMsg *PMNewViewMessage, addr types.NetAddress) error {
	epochID := newViewMsg.CSMsgCommonHeader.EpochID
	if err := p.sendQueryProposalMsg(queryHeight, queryRound, epochID, addr); err != nil {
		p.logger.Warn("send PMQueryProposal message failed", "err", err)
	}

	p.pendingList.Add(newViewMsg, addr)
	return nil
}

func (p *Pacemaker) checkPendingMessages(curHeight uint64) error {
	height := curHeight
	p.logger.Info("checkPendingMessage", "start height", height)
	for {
		pendingMsg, ok := p.pendingList.messages[height]
		if !ok {
			break
		}
		p.pacemakerMsgCh <- pendingMsg
		/*
			proposer := p.getConsensusPeerByPubkey(pm.ProposerID)
			if proposer == nil {
				p.logger.Error("can not get proposer", "height", height)
				break
			}
			p.logger.Info("processing pending list", "height", height)
			if err := p.OnReceiveProposal(pm, proposer.netAddr); err != nil {
				p.logger.Error("error happens", "height", height, "error", err)
				break
			}
		*/
		height++ //move higher
	}

	p.pendingList.CleanUpTo(height)
	return nil
}
