// Copyright (c) 2018 The VeChainmeter developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/wire"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
)

type JSONBlockSummary struct {
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
	TxsFeatures      uint32             `json:"txsFeatures"`
	StateRoot        meter.Bytes32      `json:"stateRoot"`
	ReceiptsRoot     meter.Bytes32      `json:"receiptsRoot"`
	Signer           meter.Address      `json:"signer"`
	IsTrunk          bool               `json:"isTrunk"`
	IsKBlock         bool               `json:"isKBlock"`
	LastKBlockHeight uint32             `json:"lastKBlockHeight"`
	CommitteeInfo    []*CommitteeMember `json:"committee"`
	QC               *QC                `json:"qc"`
	Nonce            uint64             `json:"nonce"`
}

type JSONCollapsedBlock struct {
	*JSONBlockSummary
	Transactions []meter.Bytes32 `json:"transactions"`
}

type JSONClause struct {
	To    *meter.Address       `json:"to"`
	Value math.HexOrDecimal256 `json:"value"`
	Token uint32               `json:"token"`
	Data  string               `json:"data"`
}

type JSONTransfer struct {
	Sender    meter.Address         `json:"sender"`
	Recipient meter.Address         `json:"recipient"`
	Amount    *math.HexOrDecimal256 `json:"amount"`
}

type JSONEvent struct {
	Address meter.Address   `json:"address"`
	Topics  []meter.Bytes32 `json:"topics"`
	Data    string          `json:"data"`
}

type JSONOutput struct {
	ContractAddress *meter.Address  `json:"contractAddress"`
	Events          []*JSONEvent    `json:"events"`
	Transfers       []*JSONTransfer `json:"transfers"`
}

type JSONEmbeddedTx struct {
	ID           meter.Bytes32       `json:"id"`
	ChainTag     byte                `json:"chainTag"`
	BlockRef     string              `json:"blockRef"`
	Expiration   uint32              `json:"expiration"`
	Clauses      []*JSONClause       `json:"clauses"`
	GasPriceCoef uint8               `json:"gasPriceCoef"`
	Gas          uint64              `json:"gas"`
	Origin       meter.Address       `json:"origin"`
	Delegator    *meter.Address      `json:"delegator"`
	Nonce        math.HexOrDecimal64 `json:"nonce"`
	DependsOn    *meter.Bytes32      `json:"dependsOn"`
	Size         uint32              `json:"size"`

	// receipt part
	GasUsed  uint64                `json:"gasUsed"`
	GasPayer meter.Address         `json:"gasPayer"`
	Paid     *math.HexOrDecimal256 `json:"paid"`
	Reward   *math.HexOrDecimal256 `json:"reward"`
	Reverted bool                  `json:"reverted"`
	Outputs  []*JSONOutput         `json:"outputs"`
}

type JSONPowBlock struct {
	Hash      string `json:"hash"`
	PrevBlock string `json:"prevBlock"`
	Reward    string `json:"reward"`
}

type JSONEpoch struct {
	EpochID   uint64          `json:"epochID"`
	PowBlocks []*JSONPowBlock `json:"powBlocks"`
	Nonce     uint64          `json:"nonce"`
}

func buildJSONEpoch(blk *block.Block) *JSONEpoch {
	txs := blk.Transactions()
	powRaws := blk.KBlockData.Data
	clauses := make([]*tx.Clause, 0)
	for _, t := range txs {
		for _, c := range t.Clauses() {
			clauses = append(clauses, c)
		}
	}

	if len(clauses) < len(powRaws) {
		return nil
	}

	jPowBlks := make([]*JSONPowBlock, 0)
	for i, powRaw := range powRaws {
		clause := clauses[i]
		powBlock := wire.MsgBlock{}
		err := powBlock.Deserialize(bytes.NewReader(powRaw))
		if err != nil {
			fmt.Println("could not deserialize msgBlock, error:", err)
		}
		jPowBlk := &JSONPowBlock{
			Hash:      powBlock.Header.BlockHash().String(),
			PrevBlock: powBlock.Header.PrevBlock.String(),
			Reward:    clause.Value().String(),
		}
		jPowBlks = append(jPowBlks, jPowBlk)
	}

	return &JSONEpoch{
		Nonce:     blk.KBlockData.Nonce,
		EpochID:   blk.CommitteeInfos.Epoch,
		PowBlocks: jPowBlks,
	}
}

type JSONExpandedBlock struct {
	*JSONBlockSummary
	Transactions []*JSONEmbeddedTx `json:"transactions"`
}

func buildJSONBlockSummary(blk *block.Block, isTrunk bool) *JSONBlockSummary {
	header := blk.Header()
	signer, _ := header.Signer()

	result := &JSONBlockSummary{
		Number:           header.Number(),
		ID:               header.ID(),
		ParentID:         header.ParentID(),
		Timestamp:        header.Timestamp(),
		TotalScore:       header.TotalScore(),
		GasLimit:         header.GasLimit(),
		GasUsed:          header.GasUsed(),
		Beneficiary:      header.Beneficiary(),
		Signer:           signer,
		Size:             uint32(blk.Size()),
		StateRoot:        header.StateRoot(),
		ReceiptsRoot:     header.ReceiptsRoot(),
		TxsRoot:          header.TxsRoot(),
		IsTrunk:          isTrunk,
		IsKBlock:         header.BlockType() == block.BLOCK_TYPE_K_BLOCK,
		LastKBlockHeight: header.LastKBlockHeight(),
	}
	var err error
	if blk.QC != nil {
		result.QC, err = convertQC(blk.QC)
		if err != nil {
			return nil
		}
		result.QC.Raw = ""
	}

	if len(blk.CommitteeInfos.CommitteeInfo) > 0 {
		result.CommitteeInfo = convertCommitteeList(blk.CommitteeInfos)
	} else {
		result.CommitteeInfo = make([]*CommitteeMember, 0)
	}
	if blk.KBlockData.Nonce > 0 {
		result.Nonce = blk.KBlockData.Nonce
	}
	return result
}

func buildJSONOutput(txID meter.Bytes32, index uint32, c *tx.Clause, o *tx.Output) *JSONOutput {
	jo := &JSONOutput{
		ContractAddress: nil,
		Events:          make([]*JSONEvent, 0, len(o.Events)),
		Transfers:       make([]*JSONTransfer, 0, len(o.Transfers)),
	}
	if c.To() == nil {
		addr := meter.CreateContractAddress(txID, index, 0)
		jo.ContractAddress = &addr
	}
	for _, e := range o.Events {
		jo.Events = append(jo.Events, &JSONEvent{
			Address: e.Address,
			Data:    hexutil.Encode(e.Data),
			Topics:  e.Topics,
		})
	}
	for _, t := range o.Transfers {
		jo.Transfers = append(jo.Transfers, &JSONTransfer{
			Sender:    t.Sender,
			Recipient: t.Recipient,
			Amount:    (*math.HexOrDecimal256)(t.Amount),
		})
	}
	return jo
}

func buildJSONEmbeddedTxs(txs tx.Transactions, receipts tx.Receipts) []*JSONEmbeddedTx {
	jTxs := make([]*JSONEmbeddedTx, 0, len(txs))
	for itx, tx := range txs {
		receipt := receipts[itx]

		clauses := tx.Clauses()
		blockRef := tx.BlockRef()
		origin, _ := tx.Signer()

		jcs := make([]*JSONClause, 0, len(clauses))
		jos := make([]*JSONOutput, 0, len(receipt.Outputs))

		for i, c := range clauses {
			jcs = append(jcs, &JSONClause{
				c.To(),
				math.HexOrDecimal256(*c.Value()),
				uint32(c.Token()),
				hexutil.Encode(c.Data()),
			})
			if !receipt.Reverted {
				jos = append(jos, buildJSONOutput(tx.ID(), uint32(i), c, receipt.Outputs[i]))
			}
		}

		jTxs = append(jTxs, &JSONEmbeddedTx{
			ID:           tx.ID(),
			ChainTag:     tx.ChainTag(),
			BlockRef:     hexutil.Encode(blockRef[:]),
			Expiration:   tx.Expiration(),
			Clauses:      jcs,
			GasPriceCoef: tx.GasPriceCoef(),
			Gas:          tx.Gas(),
			Origin:       origin,
			Nonce:        math.HexOrDecimal64(tx.Nonce()),
			DependsOn:    tx.DependsOn(),
			Size:         uint32(tx.Size()),

			GasUsed:  receipt.GasUsed,
			GasPayer: receipt.GasPayer,
			Paid:     (*math.HexOrDecimal256)(receipt.Paid),
			Reward:   (*math.HexOrDecimal256)(receipt.Reward),
			Reverted: receipt.Reverted,
			Outputs:  jos,
		})
	}
	return jTxs
}

type QC struct {
	QCHeight         uint32 `json:"qcHeight"`
	QCRound          uint32 `json:"qcRound"`
	VoterBitArrayStr string `json:"voterBitArrayStr"`
	EpochID          uint64 `json:"epochID"`
	Raw              string `json:"raw"`
}

type CommitteeMember struct {
	Index uint32 `json:"index"`
	// Name    string `json:"name"`
	NetAddr string `json:"netAddr"`
	PubKey  string `json:"pubKey"`
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
			Index: cm.CSIndex,
			// Name:    "",
			NetAddr: cm.NetAddr.IP.String(),
			PubKey:  hex.EncodeToString(cm.PubKey),
		}
	}
	return committeeList
}
