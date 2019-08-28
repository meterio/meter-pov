package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"

	//"github.com/dfinlab/meter/types"
	"net"
	"net/http"
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// reasons for new view
const (
	PROPOSE_MSG_SUBTYPE_KBLOCK        = byte(0x01)
	PROPOSE_MSG_SUBTYPE_MBLOCK        = byte(0x02)
	PROPOSE_MSG_SUBTYPE_STOPCOMMITTEE = byte(255)
)

type NewViewReason byte

const (
	HigherQCSeen NewViewReason = NewViewReason(1)
	RoundTimeout NewViewReason = NewViewReason(2)
)

// ***********************************
type TimeoutCert struct {
	TimeoutRound     uint64
	TimeoutHeight    uint64
	TimeoutCounter   uint32
	TimeoutSignature []byte
}

func newTimeoutCert(height, round uint64, counter uint32) *TimeoutCert {
	return &TimeoutCert{
		TimeoutRound:   round,
		TimeoutHeight:  height,
		TimeoutCounter: counter,
	}
}

func (tc *TimeoutCert) SigningHash() (hash meter.Bytes32) {
	hw := meter.NewBlake2b()
	rlp.Encode(hw, []interface{}{
		tc.TimeoutRound,
		tc.TimeoutHeight,
		tc.TimeoutCounter,
	})
	hw.Sum(hash[:0])
	return
}

// check a pmBlock is the extension of b_locked, max 10 hops
func (p *Pacemaker) IsExtendedFromBLocked(b *pmBlock) bool {

	i := int(0)
	tmp := b
	for i < 10 {
		if tmp == p.blockLocked {
			return true
		}
		tmp = tmp.Parent
		i++
	}
	return false
}

// find out b b' b"
func (p *Pacemaker) AddressBlock(height uint64, round uint64) *pmBlock {
	if (p.proposalMap[height] != nil) && (p.proposalMap[height].Height == height) && (p.proposalMap[height].Round == round) {
		// p.csReactor.logger.Debug("Addressed block", "height", height, "round", round)
		return p.proposalMap[height]
	}

	p.csReactor.logger.Info("Could not find out block", "height", height, "round", round)
	return nil
}

func (p *Pacemaker) receivePacemakerMsg(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var params map[string]string
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		p.csReactor.logger.Error("%v\n", err)
		respondWithJson(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	peerIP := net.ParseIP(params["peer_ip"])
	respondWithJson(w, http.StatusOK, map[string]string{"result": "success"})

	msgByteSlice, _ := hex.DecodeString(params["message"])
	msg, err := decodeMsg(msgByteSlice)
	if err != nil {
		p.csReactor.logger.Error("message decode error", "err", err)
		panic("message decode error")
	} else {
		typeName := getConcreteName(msg)
		if peerIP.String() == p.csReactor.GetMyNetAddr().IP.String() {
			p.logger.Info("Received pacemaker msg from myself", "type", typeName, "from", peerIP.String())
		} else {
			p.logger.Info("Received pacemaker msg from peer", "type", typeName, "from", peerIP.String())
		}
		p.pacemakerMsgCh <- msg
	}
}

func (p *Pacemaker) ValidateProposal(b *pmBlock) error {
	p.logger.Info("ValidateProposal", "height", b.Height, "round", b.Round, "type", b.ProposedBlockType)
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
			//return nil
		} else if parent.ProposedBlockType == StopCommitteeType {
			grandParent := p.proposalMap[b.Height-2]
			if grandParent.ProposedBlockType == KBlockType {
				p.logger.Info("The second stop committee block")
				//return nil
			} else {
				//return errParentMissing
			}
		} else {
			//return errParentMissing
		}
	}

	p.logger.Info("Validate Proposal", "block", blk.Oneliner())

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
		peers, _ = p.csReactor.GetMyPeers()
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
	for _, peer := range peers {
		hint := "Sending pacemaker msg to peer"
		if peer.netAddr.IP.String() == myNetAddr.IP.String() {
			hint = "Sending pacemaker msg to myself"
		}
		p.logger.Debug(hint, "type", typeName, "to", peer.netAddr.IP.String())

		// TODO: make this asynchornous
		peer.sendData(myNetAddr, typeName, rawMsg)
	}
	return true
}
