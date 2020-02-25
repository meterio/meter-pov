// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks

import (
	"encoding/hex"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
)

//Block block
type Block struct {
	Number           uint32             `json:"number"`
	ID               meter.Bytes32      `json:"id"`
	Size             uint32             `json:"size"`
	ParentID         meter.Bytes32      `json:"parentID"`
	Timestamp        uint64             `json:"timestamp"`
	GasLimit         uint64             `json:"gasLimit"`
	Beneficiary      meter.Address      `json:"beneficiary"`
	GasUsed          uint64             `json:"gasUsed"`
	TotalScore       uint64             `json:"totalScore"`
	TxsRoot          meter.Bytes32      `json:"txsRoot"`
	StateRoot        meter.Bytes32      `json:"stateRoot"`
	ReceiptsRoot     meter.Bytes32      `json:"receiptsRoot"`
	Signer           meter.Address      `json:"signer"`
	IsTrunk          bool               `json:"isTrunk"`
	Transactions     []meter.Bytes32    `json:"transactions"`
	IsKBlock         bool               `json:"isKBlock"`
	LastKBlockHeight uint32             `json:"lastKBlockHeight"`
	QCHeight         uint64             `json:"qcHeight"`
	QCRound          uint64             `json:"qcRound"`
	EpochID          uint64             `json:"epochID"`
	CommitteeInfo    []*CommitteeMember `json:"committeeMembers"`
}
type QC struct {
	QCHeight         uint64 `json:"qcHeight"`
	QCRound          uint64 `json:"qcRound"`
	VoterBitArrayStr string `json:"voterBitArrayStr"`
	EpochID          uint64 `json:"epochID"`
	Raw              string `json:"raw"`
}

type CommitteeMember struct {
	Index   uint32 `json:"index"`
	Name    string `json:"name"`
	NetAddr string `json:"netAddr"`
}

func convertQC(qc *block.QuorumCert) (*QC, error) {
	raw := hex.EncodeToString(qc.ToBytes())
	return &QC{
		QCHeight:         qc.QCHeight,
		QCRound:          qc.QCRound,
		VoterBitArrayStr: qc.VoterBitArrayStr,
		EpochID:          qc.EpochID,
		Raw:              raw,
	}, nil
}

func convertCommitteeList(cml block.CommitteeInfos) []*CommitteeMember {
	committeeList := make([]*CommitteeMember, len(cml.CommitteeInfo))

	for i, cm := range cml.CommitteeInfo {
		committeeList[i] = &CommitteeMember{
			Index:   cm.CSIndex,
			Name:    "",
			NetAddr: cm.NetAddr.IP.String(),
		}
	}
	return committeeList
}

func convertBlock(b *block.Block, isTrunk bool) (*Block, error) {
	if b == nil {
		return nil, nil
	}
	signer, err := b.Header().Signer()
	if err != nil {
		return nil, err
	}
	txs := b.Transactions()
	txIds := make([]meter.Bytes32, len(txs))
	for i, tx := range txs {
		txIds[i] = tx.ID()
	}

	header := b.Header()
	result := &Block{
		Number:           header.Number(),
		ID:               header.ID(),
		ParentID:         header.ParentID(),
		Timestamp:        header.Timestamp(),
		TotalScore:       header.TotalScore(),
		GasLimit:         header.GasLimit(),
		GasUsed:          header.GasUsed(),
		Beneficiary:      header.Beneficiary(),
		Signer:           signer,
		Size:             uint32(b.Size()),
		StateRoot:        header.StateRoot(),
		ReceiptsRoot:     header.ReceiptsRoot(),
		TxsRoot:          header.TxsRoot(),
		IsTrunk:          isTrunk,
		Transactions:     txIds,
		IsKBlock:         header.BlockType() == block.BLOCK_TYPE_K_BLOCK,
		LastKBlockHeight: header.LastKBlockHeight(),
	}
	if b.QC != nil {
		result.QCHeight = b.QC.QCHeight
		result.QCRound = b.QC.QCRound
		result.EpochID = b.QC.EpochID
	}

	if len(b.CommitteeInfos.CommitteeInfo) > 0 {
		result.CommitteeInfo = convertCommitteeList(b.CommitteeInfos)
	} else {
		result.CommitteeInfo = make([]*CommitteeMember, 0)
	}
	return result, nil
}
