// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package blocks

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/wire"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/tx"
	"github.com/ethereum/go-ethereum/common"
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
	Epoch            uint64             `json:"epoch"`
	KblockData       []string           `json:"kblockData"`
	PowBlocks        []*JSONPowBlock    `json:"powBlocks"`
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
	Token     uint32                `json:"token"`
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
	Hash        string `json:"hash"`
	PrevBlock   string `json:"prevBlock"`
	Beneficiary string `json:"beneficiary"`
	Height      uint32 `json:"height"`
}

type JSONEpoch struct {
	EpochID   uint64          `json:"epochID"`
	Number    uint32          `json:"number"`
	PowBlocks []*JSONPowBlock `json:"powBlocks"`
	Nonce     uint64          `json:"nonce"`
}

func buildJSONPowBlock(powRaw []byte) *JSONPowBlock {
	powBlock := wire.MsgBlock{}
	err := powBlock.Deserialize(bytes.NewReader(powRaw))
	if err != nil {
		fmt.Println("could not deserialize msgBlock, error:", err)
		return nil
	}

	var height uint32
	beneficiaryAddr := "0x"
	if len(powBlock.Transactions) == 1 && len(powBlock.Transactions[0].TxIn) == 1 {
		ss := powBlock.Transactions[0].TxIn[0].SignatureScript
		height, beneficiaryAddr = powpool.DecodeSignatureScript(ss)
	}

	jPowBlk := &JSONPowBlock{
		Hash:        powBlock.Header.BlockHash().String(),
		PrevBlock:   powBlock.Header.PrevBlock.String(),
		Beneficiary: beneficiaryAddr,
		Height:      height,
	}
	return jPowBlk
}

func buildJSONEpoch(blk *block.Block) *JSONEpoch {
	jPowBlks := make([]*JSONPowBlock, 0)
	for _, powRaw := range blk.KBlockData.Data {
		jPowBlk := buildJSONPowBlock(powRaw)
		if jPowBlk != nil {
			jPowBlks = append(jPowBlks, jPowBlk)
		}
	}

	return &JSONEpoch{
		Nonce:     blk.KBlockData.Nonce,
		EpochID:   blk.GetBlockEpoch(),
		Number:    blk.Header().Number(),
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

	var epoch uint64
	isKBlock := header.BlockType() == block.BLOCK_TYPE_K_BLOCK
	if isTrunk && isKBlock {
		epoch = blk.QC.EpochID
	} else if len(blk.CommitteeInfos.CommitteeInfo) > 0 {
		epoch = blk.CommitteeInfos.Epoch
	} else {
		epoch = blk.QC.EpochID
	}
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
		IsKBlock:         isKBlock,
		LastKBlockHeight: header.LastKBlockHeight(),
		Epoch:            epoch,
		KblockData:       make([]string, 0),
	}
	var err error
	if blk.QC != nil {
		result.QC, err = convertQC(blk.QC)
		if err != nil {
			return nil
		}
	}

	if len(blk.CommitteeInfos.CommitteeInfo) > 0 {
		result.CommitteeInfo = convertCommitteeList(blk.CommitteeInfos)
	} else {
		result.CommitteeInfo = make([]*CommitteeMember, 0)
	}
	if len(blk.KBlockData.Data) > 0 {
		raws := make([]string, 0)
		powBlks := make([]*JSONPowBlock, 0)
		for _, powRaw := range blk.KBlockData.Data {
			raws = append(raws, "0x"+hex.EncodeToString(powRaw))
			powBlk := buildJSONPowBlock(powRaw)
			if powBlk != nil {
				powBlks = append(powBlks, powBlk)
			}
		}
		result.KblockData = raws
		result.PowBlocks = powBlks
	}
	if blk.KBlockData.Nonce > 0 {
		result.Nonce = blk.KBlockData.Nonce
	}
	return result
}

func buildJSONOutput(c *tx.Clause, contractAddr *meter.Address, o *tx.Output) *JSONOutput {
	jo := &JSONOutput{
		ContractAddress: nil,
		Events:          make([]*JSONEvent, 0, len(o.Events)),
		Transfers:       make([]*JSONTransfer, 0, len(o.Transfers)),
	}
	if c.To() == nil {
		jo.ContractAddress = contractAddr
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
			Token:     uint32(t.Token),
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
		txID := tx.ID()
		nonce := tx.Nonce()

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
				contractAddr := meter.Address{}
				if meter.IsMainChainTesla(blockRef.Number()) || meter.IsTestNet() {
					contractAddr = meter.Address(meter.EthCreateContractAddress(common.Address(origin), uint32(i)+uint32(nonce)))
				} else {
					contractAddr = meter.CreateContractAddress(txID, uint32(i), 0)
				}

				jos = append(jos, buildJSONOutput(c, &contractAddr, receipt.Outputs[i]))
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
}

type CommitteeMember struct {
	Index uint32 `json:"index"`
	// Name    string `json:"name"`
	NetAddr string `json:"netAddr"`
	PubKey  string `json:"pubKey"`
}

func convertQC(qc *block.QuorumCert) (*QC, error) {
	// raw := hex.EncodeToString(qc.ToBytes())
	return &QC{
		QCHeight:         qc.QCHeight,
		QCRound:          qc.QCRound,
		VoterBitArrayStr: qc.VoterBitArrayStr,
		EpochID:          qc.EpochID,
	}, nil
}

func convertKBlockData(kdata *block.KBlockData) {
	for _, raw := range kdata.Data {
		blk := wire.MsgBlock{}
		err := blk.BtcDecode(bytes.NewReader(raw), 0, wire.BaseEncoding)
		if err != nil {
			fmt.Println("error: ", err)
		}

	}
}

func convertCommitteeList(cml block.CommitteeInfos) []*CommitteeMember {
	committeeList := make([]*CommitteeMember, len(cml.CommitteeInfo))

	for i, cm := range cml.CommitteeInfo {
		committeeList[i] = &CommitteeMember{
			Index: cm.CSIndex,
			// Name:    "",
			NetAddr: cm.NetAddr.IP.String(),
			PubKey:  base64.StdEncoding.EncodeToString(cm.PubKey),
		}
	}
	return committeeList
}
