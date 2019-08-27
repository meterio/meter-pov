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
	crypto "github.com/ethereum/go-ethereum/crypto"
)

// reasons for new view
const (
	PROPOSE_MSG_SUBTYPE_KBLOCK        = byte(0x01)
	PROPOSE_MSG_SUBTYPE_MBLOCK        = byte(0x02)
	PROPOSE_MSG_SUBTYPE_STOPCOMMITTEE = byte(255)

	NEWVIEW_HIGHER_QC_SEEN = byte(1)
	NEWVIEW_ROUND_TIMEOUT  = byte(2)
)

// ***********************************
type TimeoutCert struct {
	TimeoutRound     uint64
	TimeoutHeight    uint64
	TimeOutCounter   uint32
	TimeOutSignature []byte
}

// ****** test code ***********
/*
type PMessage struct {
	Round                   uint64
	MsgType                 byte
	QC_height               uint64
	QC_round                uint64
	Block_height            uint64
	Block_round             uint64
	Block_parent_height     uint64
	Block_parent_round      uint64
	Block_justify_QC_height uint64
	Block_justify_QC_round  uint64
}

// String returns a string representation.
func (m *PMessage) String() string {
	return fmt.Sprintf("PMessage: Round(%v), MsgtType(%v), QC_height(%v), QC_round(%v), Block_height(%v), Block_round(%v), Block_parent_height(%v), Block_parent_round(%v), Block_justify_QC_height(%v), Block_justify_QC_round(%v)",
		m.Round, m.MsgType, m.QC_height, m.QC_round, m.Block_height, m.Block_round, m.Block_parent_height,
		m.Block_parent_round, m.Block_justify_QC_height, m.Block_justify_QC_round)
}

*/

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
			return nil
		} else if parent.ProposedBlockType == StopCommitteeType {
			grandParent := p.proposalMap[b.Height-2]
			if grandParent.ProposedBlockType == KBlockType {
				p.logger.Info("The second stop committee block")
				return nil
			} else {
				return errParentMissing
			}
		} else {
			return errParentMissing
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
