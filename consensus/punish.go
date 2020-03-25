// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type StatEntry struct {
	Address    meter.Address
	Name       string
	PubKey     string
	Infraction *staking.Infraction
}

func (e StatEntry) String() string {
	return fmt.Sprintf("%s %s %s %s", e.Address.String(), e.Name, e.PubKey, e.Infraction.String())
}

func calcMissingCommittee(validators []*types.Validator, actualMembers []CommitteeMember) ([]meter.Address, error) {
	result := make([]meter.Address, 0)

	ins := make(map[ecdsa.PublicKey]bool)
	for _, m := range actualMembers {
		ins[m.PubKey] = true
	}

	for _, v := range validators {
		if _, joined := ins[v.PubKey]; !joined {
			result = append(result, v.Address)
		}
	}
	return result, nil
}

func calcMissingProposer(validators []*types.Validator, actualMembers []CommitteeMember, blocks []*block.Block) ([]meter.Address, error) {
	result := make([]meter.Address, 0)
	index := 0
	origIndex := 0
	for _, blk := range blocks {
		actualSigner, err := blk.Header().Signer()
		if err != nil {
			return result, err
		}
		// FIXME: handle cases when proposal was sent but not enough vote to form QC
		// in this case, we should punish the voter instead of the proposer
		origIndex = index
		for true {
			expectedSigner := meter.Address(crypto.PubkeyToAddress(actualMembers[index%len(actualMembers)].PubKey))
			if bytes.Compare(actualSigner.Bytes(), expectedSigner.Bytes()) == 0 {
				break
			}
			result = append(result, validators[actualMembers[index%len(actualMembers)].CSIndex].Address)

			index++
			// prevent the deadlock if actual proposer does not exist in actual committee
			if index-origIndex >= len(actualMembers) {
				break
			}
		}
	}
	return result, nil
}

func calcMissingLeader(validators []*types.Validator, actualMembers []CommitteeMember) ([]meter.Address, error) {
	result := make([]meter.Address, 0)

	actualLeader := actualMembers[0]
	index := 0
	for index < actualLeader.CSIndex {
		result = append(result, validators[index].Address)
		index++
	}
	return result, nil
}

func calcMissingVoter(validators []*types.Validator, actualMembers []CommitteeMember, blocks []*block.Block) ([]meter.Address, error) {
	result := make([]meter.Address, 0)
	for _, blk := range blocks {
		voterBitArray := blk.QC.VoterBitArray()
		for _, member := range actualMembers {
			if voterBitArray.GetIndex(member.CSIndex) == false {
				result = append(result, validators[member.CSIndex].Address)
			}
		}
	}

	return result, nil
}

func calcDoubleSigner(blocks []*block.Block) ([]meter.Address, error) {
	result := make([]meter.Address, 0)
	for _, blk := range blocks {
		// TBD: also get from evidence from 1st mblock, check the viloation
		violations := blk.QC.GetViolation()
		for _, v := range violations {
			result = append(result, v.Address)
		}
	}

	return result, nil
}

func (conR *ConsensusReactor) calcStatistics(lastKBlockHeight, height uint32) ([]*StatEntry, error) {
	if len(conR.curCommittee.Validators) == 0 {
		return nil, errors.New("committee is empty")
	}
	if len(conR.curActualCommittee) == 0 {
		return nil, errors.New("actual committee is empty")
	}
	result := make([]*StatEntry, 0)

	// data structure to save infractions
	stats := make(map[meter.Address]*StatEntry)
	for _, v := range conR.curCommittee.Validators {
		stats[v.Address] = &StatEntry{
			Address:    v.Address,
			PubKey:     hex.EncodeToString(crypto.FromECDSAPub(&v.PubKey)),
			Name:       v.Name,
			Infraction: &staking.Infraction{0, 0, 0, 0, 0},
		}
	}

	// calculate missing committee infraction
	missedCommittee, err := calcMissingCommittee(conR.curCommittee.Validators, conR.curActualCommittee)
	if err != nil {
		conR.logger.Warn("Error during missing committee calculation", "err", err)
	}
	for _, addr := range missedCommittee {
		inf := stats[addr].Infraction
		inf.MissingCommittee++
	}

	// calculate missing leader
	missedLeader, err := calcMissingLeader(conR.curCommittee.Validators, conR.curActualCommittee)
	if err != nil {
		conR.logger.Warn("Error during missing leader calculation:", "err", err)
	}
	for _, addr := range missedLeader {
		inf := stats[addr].Infraction
		inf.MissingLeader++
	}

	// fetch all the blocks
	blocks := make([]*block.Block, 0)
	h := lastKBlockHeight + 1
	for h <= height {
		blk, err := conR.chain.GetTrunkBlock(h)
		if err != nil {
			return result, err
		}
		blocks = append(blocks, blk)
		h++
	}
	// calculate missing proposer
	missedProposer, err := calcMissingProposer(conR.curCommittee.Validators, conR.curActualCommittee, blocks)
	if err != nil {
		conR.logger.Warn("Error during missing proposer calculation:", "err", err)
	}
	for _, addr := range missedProposer {
		inf := stats[addr].Infraction
		inf.MissingProposer++
	}

	// calculate missing voter
	missedVoter, err := calcMissingVoter(conR.curCommittee.Validators, conR.curActualCommittee, blocks)
	if err != nil {
		conR.logger.Warn("Error during missing voter calculation", "err", err)
	} else {
		for _, addr := range missedVoter {
			inf := stats[addr].Infraction
			inf.MissingVoter++
		}
	}

	doubleSigner, err := calcDoubleSigner(blocks)
	if err != nil {
		conR.logger.Warn("Error during missing voter calculation", "err", err)
	} else {
		for _, addr := range doubleSigner {
			inf := stats[addr].Infraction
			inf.DoubleSigner++
		}
	}

	for signer := range stats {
		entry := stats[signer]
		result = append(result, entry)
	}
	return result, nil
}

func buildStatisticsData(entry *StatEntry) (ret []byte) {
	stakingID := staking.PackCountersToBytes(entry.Infraction)
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE_STATISTICS,
		Option:     0,
		Timestamp:  uint64(time.Now().Unix()),
		Nonce:      rand.Uint64(),
		CandAddr:   entry.Address,
		CandName:   []byte(entry.Name),
		CandPubKey: []byte(entry.PubKey),
		StakingID:  *stakingID,
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		return
	}

	s := &script.Script{
		Header: script.ScriptHeader{
			Version: uint32(0),
			ModID:   script.STAKING_MODULE_ID,
		},
		Payload: payload,
	}
	data, err := rlp.EncodeToBytes(s)
	if err != nil {
		return
	}
	data = append(script.ScriptPattern[:], data...)
	prefix := []byte{0xff, 0xff, 0xff, 0xff}
	ret = append(prefix, data...)
	// fmt.Println("script Hex:", hex.EncodeToString(ret))
	return
}

// create statistics transaction
func (conR *ConsensusReactor) BuildStatisticsTx(entries []*StatEntry) *tx.Transaction {

	// fmt.Println(fmt.Sprintf("BuildStatisticsTx, entries=%v", entries))
	// statistics transaction:
	// 1. signer is nil
	// 1. located second transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(conR.chain.Tag()).
		BlockRef(tx.NewBlockRef(conR.chain.BestBlock().Header().Number() + 1)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(2100000).
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses
	for _, entry := range entries {
		data := buildStatisticsData(entry)
		builder.Clause(
			tx.NewClause(&staking.StakingModuleAddr).
				WithValue(big.NewInt(0)).
				WithToken(tx.TOKEN_METER_GOV).
				WithData(data))
		conR.logger.Info("Statistic:", "entry", entry.String())
	}

	builder.Build().IntrinsicGas()
	return builder.Build()
}
