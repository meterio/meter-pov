// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package reward

import (
	"bytes"
	"crypto/ecdsa"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
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
		Gas(meter.BaseTxGas * uint64(len(entries)+20)).
		DependsOn(nil).
		Nonce(12345678)

	//now build Clauses
	fmt.Println("Statistics Results")
	for _, entry := range entries {
		data := buildStatisticsData(entry, curEpoch)
		builder.Clause(
			tx.NewClause(&staking.StakingModuleAddr).
				WithValue(big.NewInt(0)).
				WithToken(meter.MTRG).
				WithData(data))
		logger.Debug("Statistic entry", "entry", entry.String())
		fmt.Println(entry.Name, entry.Address, entry.Infraction.String())
	}

	builder.Build().IntrinsicGas()
	return builder.Build()
}

func buildStatisticsData(entry *StatEntry, curEpoch uint32) (ret []byte) {
	extra, err := staking.PackInfractionToBytes(&entry.Infraction)
	if err != nil {
		logger.Error("packing infraction failed", "error", err.Error())
		return
	}
	body := &staking.StakingBody{
		Opcode:     staking.OP_DELEGATE_STATISTICS,
		Option:     curEpoch,
		Timestamp:  uint64(time.Now().Unix()),
		Nonce:      rand.Uint64(),
		CandAddr:   entry.Address,
		CandName:   []byte(entry.Name),
		CandPubKey: []byte(entry.PubKey),
		ExtraData:  extra,
	}
	payload, err := rlp.EncodeToBytes(body)
	if err != nil {
		logger.Error("encode stakingBody failed", "error", err.Error())
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

func ComputeMissingProposer(validators []*types.Validator, actualMembers []types.CommitteeMember, blocks []*block.Block, curEpoch uint32) ([]*missingProposerInfo, error) {
	result := make([]*missingProposerInfo, 0)
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
			info := &missingProposerInfo{
				Address: validators[actualMembers[index%len(actualMembers)].CSIndex].Address,
				Info: staking.MissingProposerInfo{
					Epoch:  curEpoch,
					Height: blk.Header().Number(),
				},
			}
			result = append(result, info)
			logger.Debug("missingPropopser", "height", info.Info.Height, "expectedSigner", expectedSigner, "actualSigner", actualSigner)
			index++
			// prevent the deadlock if actual proposer does not exist in actual committee
			if index-origIndex >= len(actualMembers) {
				break
			}
		}
		// increase index for next
		index++
	}
	return result, nil
}

func ComputeMissingLeader(validators []*types.Validator, actualMembers []types.CommitteeMember, curEpoch uint32) ([]*missingLeaderInfo, error) {
	result := make([]*missingLeaderInfo, 0)

	actualLeader := actualMembers[0]
	index := 0
	for index < actualLeader.CSIndex {
		info := &missingLeaderInfo{
			Address: validators[index].Address,
			Info: staking.MissingLeaderInfo{
				Epoch: curEpoch,
				Round: uint32(index),
			},
		}
		result = append(result, info)
		logger.Debug("missingLeader", "address", info.Address, "epoch", info.Info.Epoch)
		index++
	}
	return result, nil
}

func ComputeMissingVoter(validators []*types.Validator, actualMembers []types.CommitteeMember, blocks []*block.Block) ([]*missingVoterInfo, error) {
	result := make([]*missingVoterInfo, 0)

	for i, blk := range blocks {
		// the 1st block is first mblock, QC is for last kblock. ignore
		if i == 0 {
			continue
		}

		voterBitArray := blk.QC.VoterBitArray()
		if voterBitArray == nil {
			logger.Warn("voterBitArray is nil")
		}
		for _, member := range actualMembers {
			if voterBitArray.GetIndex(member.CSIndex) == false {
				info := &missingVoterInfo{
					Address: validators[member.CSIndex].Address,
					Info: staking.MissingVoterInfo{
						Epoch:  uint32(blk.QC.EpochID),
						Height: blk.QC.QCHeight,
					},
				}
				result = append(result, info)
				logger.Debug("calc missingVoter", "height", info.Info.Height, "address", info.Address)
			}
		}
	}

	logger.Debug("calcMissingVoter", "result", result)
	return result, nil
}

func ComputeDoubleSigner(common *types.ConsensusCommon, blocks []*block.Block, curEpoch uint32) ([]*doubleSignerInfo, error) {
	result := make([]*doubleSignerInfo, 0)
	if len(blocks) < 1 {
		return make([]*doubleSignerInfo, 0), errors.New("could not find committee info")
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
				if err != nil {
					break
				}
				sig1, err := common.GetSystem().SigFromBytes(v.Signature1)
				if err != nil {
					break
				}
				sig2, err := common.GetSystem().SigFromBytes(v.Signature2)
				if err != nil {
					break
				}
				bls.Verify(sig1, v.MsgHash, blsPK)
				bls.Verify(sig2, v.MsgHash, blsPK)

				info := &doubleSignerInfo{
					Address: v.Address,
					Info: staking.DoubleSignerInfo{
						Epoch:  curEpoch,
						Height: blk.Header().Number(),
					},
				}
				result = append(result, info)
				logger.Debug("doubleSigner", "height", info.Info.Height, "signature1", sig1, "signature2", sig2)
			}
		}
	}

	return result, nil
}

func combinePubKey(csCommon *types.ConsensusCommon, ecdsaPub *ecdsa.PublicKey, blsPub *bls.PublicKey) string {
	ecdsaPubBytes := crypto.FromECDSAPub(ecdsaPub)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)

	blsPubBytes := csCommon.GetSystem().PubKeyToBytes(*blsPub)
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	return strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
}

func findInActualCommittee(actualCommittee []types.CommitteeMember, addr meter.Address) int {
	for i, m := range actualCommittee {
		memberAddr := meter.Address(crypto.PubkeyToAddress(m.PubKey))
		if bytes.Compare(memberAddr.Bytes(), addr.Bytes()) == 0 {
			return i
		}
	}
	return -1
}

func ComputeStatistics(lastKBlockHeight, height uint32, chain *chain.Chain, curCommittee *types.ValidatorSet, curActualCommittee []types.CommitteeMember, csCommon *types.ConsensusCommon, newCommittee bool, curEpoch uint32) ([]*StatEntry, error) {
	logger.Info("calcStatistics", "height", height, "lastKblockHeight", lastKBlockHeight)
	if len(curCommittee.Validators) == 0 {
		return nil, errors.New("committee is empty")
	}
	if len(curActualCommittee) == 0 {
		return nil, errors.New("actual committee is empty")
	}
	result := make([]*StatEntry, 0)

	// data structure to save infractions
	stats := make(map[meter.Address]*StatEntry)
	for _, v := range curCommittee.Validators {
		stats[v.Address] = &StatEntry{
			Address: v.Address,
			PubKey:  combinePubKey(csCommon, &v.PubKey, &v.BlsPubKey),
			Name:    v.Name,
		}
	}

	// calculate missing leader
	missedLeader, err := ComputeMissingLeader(curCommittee.Validators, curActualCommittee, curEpoch)
	if err != nil {
		logger.Warn("Error during missing leader calculation:", "err", err)
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
	for h < (height - 2) {
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
	logger.Debug("missing proposer:", "epoch", curEpoch, "newCommittee", newCommittee)
	if newCommittee == true {
		missedProposer, err := ComputeMissingProposer(curCommittee.Validators, curActualCommittee, blocks, curEpoch)
		if err != nil {
			logger.Warn("Error during missing proposer calculation:", "err", err)
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
					indexi := findInActualCommittee(curActualCommittee, pi.Address)
					indexj := findInActualCommittee(curActualCommittee, pj.Address)
					if indexi < indexj || (indexi == len(curActualCommittee)-1 && indexj == 0) {
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
					fmt.Println("exempt missing proposer: ", m.Address, "epoch:", m.Info.Epoch, "height:", m.Info.Height)
					for k := i + 1; k < j; k++ {
						mk := missedProposer[k]
						fmt.Println("followed by:", mk.Address, "epoch:", mk.Info.Epoch, "height:", mk.Info.Epoch)
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
		
	}

	// calculate missing voter
	// currently do not calc the missingVoter. Because signature aggreator skips the votes after the count reaches
	// to 2/3. So missingVoter counting is inacurate and causes the false alarm.
	/****
	missedVoter, err := conR.calcMissingVoter(conR.curCommittee.Validators, conR.curActualCommittee, blocks)
	if err != nil {
		logger.Warn("Error during missing voter calculation", "err", err)
	} else {
		for _, m := range missedVoter {
			inf := &stats[m.Address].Infraction
			inf.MissingVoters.Counter++
			minfo := &m.Info
			inf.MissingVoters.Info = append(inf.MissingVoters.Info, minfo)
		}
	}
	***/

	doubleSigner, err := ComputeDoubleSigner(csCommon, blocks, curEpoch)
	if err != nil {
		logger.Warn("Error during missing voter calculation", "err", err)
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

	logger.Info("calc statistics results", "result", result)
	return result, nil
}
