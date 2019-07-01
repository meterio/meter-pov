package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	//"github.com/dfinlab/meter/types"
	"net"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
)

func (q *QuorumCert) Assign(dst, src *QuorumCert) error {
	dst.QCHeight = src.QCHeight
	dst.QCRound = src.QCRound
	dst.QCNode = src.QCNode
	/********
	dst.proposalVoterBitArray = src.proposalVoterBitArray
	dst.proposalVoterSig = src.proposalVoterSig
	dst.proposalVoterPubKey = src.proposalVoterPubKey
	dst.proposalVoterMsgHash = src.proposalVoterMsgHash
	dst.proposalVoterAggSig = src.proposalVoterAggSig
	dst.proposalVoterNum = src.proposalVoterNum
	**************/
	return nil
}

func (b *pmBlock) Assign(dst, src *pmBlock) error {
	if dst == nil {
		dst = &pmBlock{Height: 0, Round: 0}
	}
	dst.Height = src.Height
	dst.Round = src.Round
	dst.Parent = src.Parent
	dst.Justify = src.Justify
	/**********
	dst.ProposedBlockInfo = src.ProposedBlockInfo
	dst.ProposedBlock = src.ProposedBlock
	dst.ProposedBlockType = src.ProposedBlockType
	********/
	return nil
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

// ****** test code ***********
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

func (p *Pacemaker) Send(cm CommitteeMember, m []byte) error {
	myNetAddr := p.csReactor.curCommittee.Validators[p.csReactor.curCommitteeIndex].NetAddr
	payload := map[string]interface{}{
		"message": hex.EncodeToString(m),
		"peer_ip": myNetAddr.IP.String(),
		//"peer_id":   string(myNetAddr.ID),
		"peer_port": string(myNetAddr.Port),
	}

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		panic("Failed to marshal message dict to json string")
		return err
	}

	var netClient = &http.Client{
		Timeout: time.Second * 2,
	}
	resp, err := netClient.Post("http://"+cm.NetAddr.IP.String()+":8670/peer", "application/json", bytes.NewBuffer(jsonStr))
	if err != nil {
		p.csReactor.logger.Error("Failed to send message to peer", "peer", cm.NetAddr.IP.String(), "err", err)
		return err
	}
	p.csReactor.logger.Info("Sent consensus message to peer", "peer", cm.NetAddr.IP.String(), "size", len(m))
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return nil
}

func (p *Pacemaker) sendMsg(round uint64, msgType byte, qc *QuorumCert, b *pmBlock) error {
	m := &PMessage{
		Round:                   round,
		MsgType:                 msgType,
		QC_height:               qc.QCHeight,
		QC_round:                qc.QCRound,
		Block_height:            b.Height,
		Block_round:             b.Round,
		Block_parent_height:     b.Parent.Height,
		Block_parent_round:      b.Parent.Round,
		Block_justify_QC_height: b.Justify.QCHeight,
		Block_justify_QC_round:  b.Justify.QCRound,
	}

	msgByte, err := rlp.EncodeToBytes(m)
	if err != nil {
		fmt.Println("panic:", err)
		panic("message encode failed")
	}

	to := p.csReactor.getRoundProposer(int(round))
	p.Send(to, msgByte)
	return nil
}

// everybody in committee include myself
func (p *Pacemaker) broadcastMsg(round uint64, msgType byte, qc *QuorumCert, b *pmBlock) error {
	m := &PMessage{
		Round:                   round,
		MsgType:                 msgType,
		QC_height:               qc.QCHeight,
		QC_round:                qc.QCRound,
		Block_height:            b.Height,
		Block_round:             b.Round,
		Block_parent_height:     b.Parent.Height,
		Block_parent_round:      b.Parent.Round,
		Block_justify_QC_height: b.Justify.QCHeight,
		Block_justify_QC_round:  b.Justify.QCRound,
	}

	msgByte, err := rlp.EncodeToBytes(m)
	if err != nil {
		fmt.Println("panic:", err)
		panic("message encode failed")
	}

	for _, cm := range p.csReactor.curActualCommittee {
		p.Send(cm, msgByte)
	}
	return nil
}

// find out b b' b"
func (p *Pacemaker) AddressBlock(height uint64, round uint64) *pmBlock {
	if (p.block != nil) && (p.block.Height == height) && (p.block.Round == round) {
		p.csReactor.logger.Info("addressed b", "height", height, "round", round)
		return p.block
	}
	if (p.blockPrime != nil) && (p.blockPrime.Height == height) && (p.blockPrime.Round == round) {
		p.csReactor.logger.Info("addressed b Prime", "height", height, "round", round)
		return p.blockPrime
	}
	if (p.blockPrimePrime != nil) && (p.blockPrimePrime.Height == height) && (p.blockPrimePrime.Round == round) {
		p.csReactor.logger.Info("addressed b PrimePrime", "height", height, "round", round)
		return p.blockPrimePrime
	}

	p.csReactor.logger.Info("Could not find out block", "height", height, "round", round)
	return nil
}

func (p *Pacemaker) decodeMsg(msg []byte) (error, *PMessage) {
	m := &PMessage{}
	if err := rlp.DecodeBytes(msg, m); err != nil {
		fmt.Println("Decode message failed", err)
		return errors.New("decode message failed"), &PMessage{}
	}
	return nil, m
}

func (p *Pacemaker) Receive(m *PMessage) error {

	// receives proposal message, block is new one. parent is one of (b,b',b")
	if m.MsgType == PACEMAKER_MSG_PROPOSAL {
		parent := p.AddressBlock(m.Block_parent_height, m.Block_parent_round)
		if parent == nil {
			return errors.New("can not address parent")
		}

		qcNode := p.AddressBlock(m.Block_justify_QC_height, m.Block_justify_QC_round)
		if qcNode == nil {
			return errors.New("can not address qcNode")
		}

		justify := &QuorumCert{
			QCHeight: m.Block_justify_QC_height,
			QCRound:  m.Block_justify_QC_round,
			QCNode:   qcNode,
		}

		b := &pmBlock{
			Height:  m.Block_height,
			Round:   m.Block_round,
			Parent:  parent,
			Justify: justify,
		}
		return p.OnReceiveProposal(b)
	} else if m.MsgType == PACEMAKER_MSG_VOTE {
		// must be in (b, b', b")
		b := p.AddressBlock(m.Block_height, m.Block_round)
		if b == nil {
			return errors.New("can not address block")
		}

		if (b.Parent.Height != m.Block_parent_height) ||
			(b.Parent.Round != m.Block_parent_round) ||
			(b.Justify.QCHeight != m.Block_justify_QC_height) ||
			(b.Justify.QCRound != m.Block_justify_QC_round) {
			return errors.New("mismatch, something wrong")
		}
		return p.OnReceiveVote(b)
	} else if m.MsgType == PACEMAKER_MSG_NEWVIEW {
		qcNode := p.AddressBlock(m.QC_height, m.QC_round)
		if qcNode == nil {
			return errors.New("can not address qcNode")
		}
		qc := &QuorumCert{
			QCHeight: m.QC_height,
			QCRound:  m.QC_round,
			QCNode:   qcNode,
		}
		return p.OnRecieveNewView(qc)
	} else {
		return errors.New("unknown pacemaker message type")
	}
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

	msgByteSlice, _ := hex.DecodeString(params["message"])
	err, message := p.decodeMsg(msgByteSlice)
	if err != nil {
		p.csReactor.logger.Error("message decode error", err)
		panic("message decode error")
	} else {
		p.csReactor.logger.Info("receive pacemaker msg from", "IP", peerIP, "msgType", message.MsgType)
		p.Receive(message)
	}
	respondWithJson(w, http.StatusOK, map[string]string{"result": "success"})

}
