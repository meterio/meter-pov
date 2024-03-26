// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package governor

import (
	"bytes"
	"crypto/ecdsa"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/params"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/types"
)

// create statistics transaction
func BuildStatisticsTx(entries []*StatEntry, chainTag byte, bestNum uint32, curEpoch uint32) *tx.Transaction {

	// fmt.Println(fmt.Sprintf("BuildStatisticsTx, entries=%v", entries))
	// statistics transaction:
	// 1. signer is nil
	// 1. located second transaction in kblock.
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestNum + 1)).
		Expiration(720).
		GasPriceCoef(0).
		// Gas(meter.BaseTxGas * uint64(len(entries)+20)).
		DependsOn(nil).
		Nonce(12345678)
	gas := meter.TxGas + meter.ClauseGas*uint64(len(entries)) + meter.BaseTxGas /* buffer */

	//now build Clauses
	for _, entry := range entries {
		data := buildStatisticsData(entry, curEpoch)
		gas += uint64(len(data)) * params.TxDataNonZeroGas
		builder.Clause(
			tx.NewClause(&meter.StakingModuleAddr).
				WithValue(big.NewInt(0)).
				WithToken(meter.MTRG).
				WithData(data))
		slog.Debug("Statistic entry", "entry", entry.String())
		slog.Info(fmt.Sprintf("Stats entry in tx on %s", entry.Name), "addr", entry.Address, "infraction", entry.Infraction.String())
	}
	builder.Gas(gas)

	builder.Build().IntrinsicGas()
	return builder.Build()
}

func buildStatisticsData(entry *StatEntry, curEpoch uint32) (ret []byte) {
	extra, err := meter.PackInfractionToBytes(&entry.Infraction)
	if err != nil {
		slog.Error("packing infraction failed", "error", err.Error())
		return
	}
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE_STATISTICS,
		Option:     curEpoch,
		Timestamp:  0,
		Nonce:      0,
		CandAddr:   entry.Address,
		CandName:   []byte(entry.Name),
		CandPubKey: []byte(entry.PubKey),
		ExtraData:  extra,
	}
	ret, _ = script.EncodeScriptData(body)
	return
}

func ComputeMissingProposer(validators []*types.Validator, blocks []*block.Block, curEpoch uint32) ([]*missingProposerInfo, error) {
	result := make([]*missingProposerInfo, 0)
	index := 0
	origIndex := 0
	for _, blk := range blocks {
		actualSigner, err := blk.Signer()
		if err != nil {
			return result, err
		}
		// FIXME: handle cases when proposal was sent but not enough vote to form QC
		// in this case, we should punish the voter instead of the proposer
		origIndex = index
		for true {
			expectedSigner := meter.Address(crypto.PubkeyToAddress(validators[index%len(validators)].PubKey))
			if bytes.Compare(actualSigner.Bytes(), expectedSigner.Bytes()) == 0 {
				break
			}
			info := &missingProposerInfo{
				Address: validators[index%len(validators)].Address,
				Info: meter.MissingProposerInfo{
					Epoch:  curEpoch,
					Height: blk.Number(),
				},
			}
			result = append(result, info)
			slog.Debug("missingPropopser", "height", info.Info.Height, "expectedSigner", expectedSigner, "actualSigner", actualSigner)
			index++
			// prevent the deadlock if actual proposer does not exist in actual committee
			if index-origIndex >= len(validators) {
				break
			}
		}
		// increase index for next
		index++
	}
	return result, nil
}

func ComputeMissingLeader(validators []*types.Validator, curEpoch uint32) ([]*missingLeaderInfo, error) {
	result := make([]*missingLeaderInfo, 0)

	// actualLeader := validators[0]
	index := 0
	for index < 0 {
		info := &missingLeaderInfo{
			Address: validators[index].Address,
			Info: meter.MissingLeaderInfo{
				Epoch: curEpoch,
				Round: uint32(index),
			},
		}
		result = append(result, info)
		slog.Debug("missingLeader", "address", info.Address, "epoch", info.Info.Epoch)
		index++
	}
	return result, nil
}

func ComputeMissingVoter(validators []*types.Validator, blocks []*block.Block) ([]*missingVoterInfo, error) {
	result := make([]*missingVoterInfo, 0)

	for i, blk := range blocks {
		// the 1st block is first mblock, QC is for last kblock. ignore
		if i == 0 {
			continue
		}

		voterBitArray := blk.QC.VoterBitArray()
		if voterBitArray == nil {
			slog.Warn("voterBitArray is nil")
		}
		for index, _ := range validators {
			if voterBitArray.GetIndex(index) == false {
				info := &missingVoterInfo{
					Address: validators[index].Address,
					Info: meter.MissingVoterInfo{
						Epoch:  uint32(blk.QC.EpochID),
						Height: blk.QC.QCHeight,
					},
				}
				result = append(result, info)
				slog.Debug("calc missingVoter", "height", info.Info.Height, "address", info.Address)
			}
		}
	}

	slog.Debug("calcMissingVoter", "result", result)
	return result, nil
}

func ComputeDoubleSigner(common *types.BlsCommon, blocks []*block.Block, curEpoch uint32) ([]*doubleSignerInfo, error) {
	result := make([]*doubleSignerInfo, 0)
	if len(blocks) < 1 {
		return make([]*doubleSignerInfo, 0), errors.New("not enough blocks")
	}
	committeeInfo := blocks[0].CommitteeInfos.CommitteeInfo
	if len(committeeInfo) <= 0 {
		return make([]*doubleSignerInfo, 0), errors.New("could not find committee info")
	}
	for _, blk := range blocks {
		violations := blk.QC.GetViolation()
		// TBD: also get from evidence from 1st mblock, check the viloation
		for _, v := range violations {
			if v.Index < len(committeeInfo) {
				blsPKBytes := committeeInfo[v.Index].CSPubKey
				blsPK, err := common.GetSystem().PubKeyFromBytes(blsPKBytes)
				defer blsPK.Free()
				if err != nil {
					break
				}
				sig1, err := common.GetSystem().SigFromBytes(v.Signature1)
				defer sig1.Free()
				if err != nil {
					break
				}
				sig2, err := common.GetSystem().SigFromBytes(v.Signature2)
				defer sig2.Free()
				if err != nil {
					break
				}
				bls.Verify(sig1, v.MsgHash, blsPK)
				bls.Verify(sig2, v.MsgHash, blsPK)

				info := &doubleSignerInfo{
					Address: v.Address,
					Info: meter.DoubleSignerInfo{
						Epoch:  curEpoch,
						Height: blk.Number(),
					},
				}
				result = append(result, info)
				slog.Debug("doubleSigner", "height", info.Info.Height, "signature1", sig1, "signature2", sig2)
			}
		}
	}

	return result, nil
}

func combinePubKey(blsCommon *types.BlsCommon, ecdsaPub *ecdsa.PublicKey, blsPub *bls.PublicKey) string {
	ecdsaPubBytes := crypto.FromECDSAPub(ecdsaPub)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)

	blsPubBytes := blsCommon.GetSystem().PubKeyToBytes(*blsPub)
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	return strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
}

func findInActualCommittee(actualCommittee []*types.Validator, addr meter.Address) int {
	for i, m := range actualCommittee {
		memberAddr := meter.Address(crypto.PubkeyToAddress(m.PubKey))
		if bytes.Compare(memberAddr.Bytes(), addr.Bytes()) == 0 {
			return i
		}
	}
	return -1
}

func ComputeStatistics(lastKBlockHeight, height uint32, chain *chain.Chain, committee []*types.Validator, blsCommon *types.BlsCommon, calcStatsTx bool, curEpoch uint32) ([]*StatEntry, error) {
	if len(committee) == 0 {
		return nil, errors.New("committee is empty")
	}
	if len(committee) == 0 {
		return nil, errors.New("actual committee is empty")
	}
	result := make([]*StatEntry, 0)

	// data structure to save infractions
	stats := make(map[meter.Address]*StatEntry)
	for _, v := range committee {
		stats[v.Address] = &StatEntry{
			Address: v.Address,
			PubKey:  combinePubKey(blsCommon, &v.PubKey, &v.BlsPubKey),
			Name:    v.Name,
		}
	}

	// calculate missing leader
	missedLeader, err := ComputeMissingLeader(committee, curEpoch)
	if err != nil {
		slog.Warn("Error during missing leader calculation:", "err", err)
	}
	for _, m := range missedLeader {
		inf := &stats[m.Address].Infraction
		inf.MissingLeaders.Counter++
		minfo := &m.Info
		inf.MissingLeaders.Info = append(inf.MissingLeaders.Info, minfo)
	}

	// fetch all the blocks
	// currently we are building the kblock height. Only height - 3 are available in chain
	blocks := make([]*block.Block, 0)
	h := lastKBlockHeight + 1
	for h < (height-2) && h < lastKBlockHeight+10000 {
		blk, err := chain.GetTrunkBlock(h)
		if err != nil {
			return result, err
		}
		blocks = append(blocks, blk)
		h++
	}

	// Do not do statistics if this committee is replayed.
	// TBD: building the kblock means committee meber, so can get
	// the last 2 blocks from pacemaker's proposalMap

	// calculate missing proposer
	slog.Debug("missing proposer:", "epoch", curEpoch, "calcStatsTx", calcStatsTx)
	// fmt.Println("cur Actual Committee: ", len(committee))
	// for _, m := range committee {
	// 	fmt.Println("Member: ", m.CSIndex, m.Name, m.NetAddr.String())
	// }
	if calcStatsTx == true {
		missedProposer, err := ComputeMissingProposer(committee, blocks, curEpoch)
		if err != nil {
			slog.Warn("Error during missing proposer calculation:", "err", err)
		}
		// sort all missed proposer infraction in this order
		// epoch ascend, height ascend, actual committee index ascend
		sort.SliceStable(missedProposer, func(i, j int) bool {
			pi := missedProposer[i]
			pj := missedProposer[j]

			if pi.Info.Epoch < pj.Info.Epoch {
				return true
			}
			if pi.Info.Height < pj.Info.Height {
				return true
			}
			if pi.Info.Epoch == pj.Info.Epoch && pi.Info.Height == pj.Info.Height {
				indexi := findInActualCommittee(committee, pi.Address)
				indexj := findInActualCommittee(committee, pj.Address)
				if indexi < indexj || (indexi == len(committee)-1 && indexj == 0) {
					return true
				}
			}
			return false
		})

		i := 0
		for i < len(missedProposer) {
			m := missedProposer[i]

			// calculate the count for same (epoch, height)
			j := i + 1
			for ; j < len(missedProposer) && missedProposer[j].Info.Epoch == m.Info.Epoch && missedProposer[j].Info.Height == m.Info.Height; j++ {
			}
			length := j - i

			// if length > 1, append infractions except for the first missing proposer
			if length > 1 {
				slog.Debug("exempt missing proposer", "addr", m.Address, "epoch", m.Info.Epoch, "height", m.Info.Height)
				for k := i + 1; k < j; k++ {
					mk := missedProposer[k]
					slog.Debug("followed by", "addr", mk.Address, "epoch", mk.Info.Epoch, "height", mk.Info.Height)
					inf := &stats[mk.Address].Infraction
					inf.MissingProposers.Counter++
					minfo := &m.Info
					inf.MissingProposers.Info = append(inf.MissingProposers.Info, minfo)
				}
				i = j
			} else {
				// otherwise, append the current infraction
				inf := &stats[m.Address].Infraction
				inf.MissingProposers.Counter++
				minfo := &m.Info
				inf.MissingProposers.Info = append(inf.MissingProposers.Info, minfo)
				i = i + 1
			}

		}

	} else {
		slog.Debug("skip missing proposer calculation", "calcStatsTx", calcStatsTx)
	}

	// calculate missing voter
	// currently do not calc the missingVoter. Because signature aggreator skips the votes after the count reaches
	// to 2/3. So missingVoter counting is inacurate and causes the false alarm.
	/****
	missedVoter, err := r.calcMissingVoter(r.committee, r.committee, blocks)
	if err != nil {
		slog.Warn("Error during missing voter calculation", "err", err)
	} else {
		for _, m := range missedVoter {
			inf := &stats[m.Address].Infraction
			inf.MissingVoters.Counter++
			minfo := &m.Info
			inf.MissingVoters.Info = append(inf.MissingVoters.Info, minfo)
		}
	}
	***/

	doubleSigner, err := ComputeDoubleSigner(blsCommon, blocks, curEpoch)
	if err != nil {
		slog.Warn("Error during missing voter calculation", "err", err)
	} else {
		for _, m := range doubleSigner {
			inf := &stats[m.Address].Infraction
			inf.DoubleSigners.Counter++
			minfo := &m.Info
			inf.DoubleSigners.Info = append(inf.DoubleSigners.Info, minfo)
		}
	}

	for signer := range stats {
		inf := &stats[signer].Infraction
		// remove non-changed entries
		if (inf.MissingLeaders.Counter == 0) && (inf.MissingProposers.Counter == 0) &&
			(inf.MissingVoters.Counter == 0) && (inf.DoubleSigners.Counter == 0) {
			continue
		}
		result = append(result, stats[signer])
	}

	// slog.Info("calc statistics results", "result", result)
	return result, nil
}
