// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks

import (
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
)

//Block block
type Block struct {
	Number           uint32          `json:"number"`
	ID               meter.Bytes32   `json:"id"`
	Size             uint32          `json:"size"`
	ParentID         meter.Bytes32   `json:"parentID"`
	Timestamp        uint64          `json:"timestamp"`
	GasLimit         uint64          `json:"gasLimit"`
	Beneficiary      meter.Address   `json:"beneficiary"`
	GasUsed          uint64          `json:"gasUsed"`
	TotalScore       uint64          `json:"totalScore"`
	TxsRoot          meter.Bytes32   `json:"txsRoot"`
	StateRoot        meter.Bytes32   `json:"stateRoot"`
	ReceiptsRoot     meter.Bytes32   `json:"receiptsRoot"`
	Signer           meter.Address   `json:"signer"`
	IsTrunk          bool            `json:"isTrunk"`
	Transactions     []meter.Bytes32 `json:"transactions"`
	IsKBlock         bool            `json:"isKBlock"`
	LastKBlockHeight uint32          `json:"lastKBlockHeight"`
	QCHeight         uint64          `json:"qcHeight"`
	QCRound          uint64          `json:"qcRound"`
	EpochID          uint64          `json:"epochID"`
	CommitteeInfo    string          `json:"committeeInfo"`
}
type QC struct {
	QCHeight         uint64 `json:"qcHeight"`
	QCRound          uint64 `json:"qcRound"`
	VoterBitArrayStr string `json:"voterBitArrayStr"`
	EpochID          uint64 `json:"epochID"`
}

func convertQC(qc *block.QuorumCert) (*QC, error) {
	return &QC{
		QCHeight:         qc.QCHeight,
		QCRound:          qc.QCRound,
		VoterBitArrayStr: qc.VoterBitArrayStr,
		EpochID:          qc.EpochID,
	}, nil
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
		result.CommitteeInfo = b.CommitteeInfos.String()
	}
	return result, nil
}
