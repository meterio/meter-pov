// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	//"crypto/ecdsa"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/builtin"

	//"github.com/vechain/thor/chain"
	"github.com/vechain/thor/poa"
	"github.com/vechain/thor/runtime"
	"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"

	//"github.com/vechain/thor/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/vechain/thor/xenv"
)

// Consensus check whether the block is verified,
// and predicate which trunk it belong to.
/**********************
type Consensus struct {
	chain        *chain.Chain
	stateCreator *state.Creator
}

// New create a Consensus instance.
func New(chain *chain.Chain, stateCreator *state.Creator) *Consensus {
	return &Consensus{
		chain:        chain,
		stateCreator: stateCreator}
}
******************/
// Process process a block.
func (c *ConsensusReactor) Process(blk *block.Block, nowTimestamp uint64) (*state.Stage, tx.Receipts, error) {
	header := blk.Header()

	if _, err := c.chain.GetBlockHeader(header.ID()); err != nil {
		if !c.chain.IsNotFound(err) {
			return nil, nil, err
		}
	} else {
		return nil, nil, errKnownBlock
	}

	parentHeader, err := c.chain.GetBlockHeader(header.ParentID())
	if err != nil {
		if !c.chain.IsNotFound(err) {
			return nil, nil, err
		}
		return nil, nil, errParentMissing
	}

	state, err := c.stateCreator.NewState(parentHeader.StateRoot())
	if err != nil {
		return nil, nil, err
	}

	stage, receipts, err := c.validate(state, blk, parentHeader, nowTimestamp)
	if err != nil {
		return nil, nil, err
	}

	return stage, receipts, nil
}

func (c *ConsensusReactor) NewRuntimeForReplay(header *block.Header) (*runtime.Runtime, error) {
	signer, err := header.Signer()
	if err != nil {
		return nil, err
	}
	parentHeader, err := c.chain.GetBlockHeader(header.ParentID())
	if err != nil {
		if !c.chain.IsNotFound(err) {
			return nil, err
		}
		return nil, errParentMissing
	}
	state, err := c.stateCreator.NewState(parentHeader.StateRoot())
	if err != nil {
		return nil, err
	}
	if err := c.validateProposer(header, parentHeader, state); err != nil {
		return nil, err
	}

	return runtime.New(
		c.chain.NewSeeker(header.ParentID()),
		state,
		&xenv.BlockContext{
			Beneficiary: header.Beneficiary(),
			Signer:      signer,
			Number:      header.Number(),
			Time:        header.Timestamp(),
			GasLimit:    header.GasLimit(),
			TotalScore:  header.TotalScore(),
		}), nil
}

func (c *ConsensusReactor) validate(
	state *state.State,
	block *block.Block,
	parentHeader *block.Header,
	nowTimestamp uint64,
) (*state.Stage, tx.Receipts, error) {
	header := block.Header()

	if err := c.validateBlockHeader(header, parentHeader, nowTimestamp); err != nil {
		return nil, nil, err
	}

	if err := c.validateProposer(header, parentHeader, state); err != nil {
		return nil, nil, err
	}

	if err := c.validateBlockBody(block); err != nil {
		return nil, nil, err
	}

	stage, receipts, err := c.verifyBlock(block, state)
	if err != nil {
		return nil, nil, err
	}

	return stage, receipts, nil
}

func (c *ConsensusReactor) validateBlockHeader(header *block.Header, parent *block.Header, nowTimestamp uint64) error {
	if header.Timestamp() <= parent.Timestamp() {
		return consensusError(fmt.Sprintf("block timestamp behind parents: parent %v, current %v", parent.Timestamp(), header.Timestamp()))
	}

	if (header.Timestamp()-parent.Timestamp())%thor.BlockInterval != 0 {
		return consensusError(fmt.Sprintf("block interval not rounded: parent %v, current %v", parent.Timestamp(), header.Timestamp()))
	}

	if header.Timestamp() > nowTimestamp+thor.BlockInterval {
		return errFutureBlock
	}

	if !block.GasLimit(header.GasLimit()).IsValid(parent.GasLimit()) {
		return consensusError(fmt.Sprintf("block gas limit invalid: parent %v, current %v", parent.GasLimit(), header.GasLimit()))
	}

	if header.GasUsed() > header.GasLimit() {
		return consensusError(fmt.Sprintf("block gas used exceeds limit: limit %v, used %v", header.GasLimit(), header.GasUsed()))
	}

	if header.TotalScore() <= parent.TotalScore() {
		return consensusError(fmt.Sprintf("block total score invalid: parent %v, current %v", parent.TotalScore(), header.TotalScore()))
	}

	return nil
}

func (c *ConsensusReactor) validateProposer(header *block.Header, parent *block.Header, st *state.State) error {
	signer, err := header.Signer()
	if err != nil {
		return consensusError(fmt.Sprintf("block signer unavailable: %v", err))
	}

	authority := builtin.Authority.Native(st)
	endorsement := builtin.Params.Native(st).Get(thor.KeyProposerEndorsement)

	candidates := authority.Candidates(endorsement, thor.MaxBlockProposers)
	proposers := make([]poa.Proposer, 0, len(candidates))
	for _, c := range candidates {
		proposers = append(proposers, poa.Proposer{
			Address: c.NodeMaster,
			Active:  c.Active,
		})
	}

	sched, err := poa.NewScheduler(signer, proposers, parent.Number(), parent.Timestamp())
	if err != nil {
		return consensusError(fmt.Sprintf("block signer invalid: %v %v", signer, err))
	}

	if !sched.IsTheTime(header.Timestamp()) {
		return consensusError(fmt.Sprintf("block timestamp unscheduled: t %v, s %v", header.Timestamp(), signer))
	}

	updates, score := sched.Updates(header.Timestamp())
	if parent.TotalScore()+score != header.TotalScore() {
		return consensusError(fmt.Sprintf("block total score invalid: want %v, have %v", parent.TotalScore()+score, header.TotalScore()))
	}

	for _, proposer := range updates {
		authority.Update(proposer.Address, proposer.Active)
	}

	return nil
}

func (c *ConsensusReactor) validateBlockBody(blk *block.Block) error {
	header := blk.Header()
	txs := blk.Transactions()
	if header.TxsRoot() != txs.RootHash() {
		return consensusError(fmt.Sprintf("block txs root mismatch: want %v, have %v", header.TxsRoot(), txs.RootHash()))
	}

	for _, tx := range txs {
		if _, err := tx.Signer(); err != nil {
			return consensusError(fmt.Sprintf("tx signer unavailable: %v", err))
		}

		switch {
		case tx.ChainTag() != c.chain.Tag():
			return consensusError(fmt.Sprintf("tx chain tag mismatch: want %v, have %v", c.chain.Tag(), tx.ChainTag()))
		case header.Number() < tx.BlockRef().Number():
			return consensusError(fmt.Sprintf("tx ref future block: ref %v, current %v", tx.BlockRef().Number(), header.Number()))
		case tx.IsExpired(header.Number()):
			return consensusError(fmt.Sprintf("tx expired: ref %v, current %v, expiration %v", tx.BlockRef().Number(), header.Number(), tx.Expiration()))
		case tx.HasReservedFields():
			return consensusError(fmt.Sprintf("tx reserved fields not empty"))
		}
	}

	return nil
}

func (c *ConsensusReactor) verifyBlock(blk *block.Block, state *state.State) (*state.Stage, tx.Receipts, error) {
	var totalGasUsed uint64
	txs := blk.Transactions()
	receipts := make(tx.Receipts, 0, len(txs))
	processedTxs := make(map[thor.Bytes32]bool)
	header := blk.Header()
	signer, _ := header.Signer()
	rt := runtime.New(
		c.chain.NewSeeker(header.ParentID()),
		state,
		&xenv.BlockContext{
			Beneficiary: header.Beneficiary(),
			Signer:      signer,
			Number:      header.Number(),
			Time:        header.Timestamp(),
			GasLimit:    header.GasLimit(),
			TotalScore:  header.TotalScore(),
		})

	findTx := func(txID thor.Bytes32) (found bool, reverted bool, err error) {
		if reverted, ok := processedTxs[txID]; ok {
			return true, reverted, nil
		}
		meta, err := c.chain.GetTransactionMeta(txID, header.ParentID())
		if err != nil {
			if c.chain.IsNotFound(err) {
				return false, false, nil
			}
			return false, false, err
		}
		return true, meta.Reverted, nil
	}

	for _, tx := range txs {
		// check if tx existed
		if found, _, err := findTx(tx.ID()); err != nil {
			return nil, nil, err
		} else if found {
			return nil, nil, consensusError("tx already exists")
		}

		// check depended tx
		if dep := tx.DependsOn(); dep != nil {
			found, reverted, err := findTx(*dep)
			if err != nil {
				return nil, nil, err
			}
			if !found {
				return nil, nil, consensusError("tx dep broken")
			}

			if reverted {
				return nil, nil, consensusError("tx dep reverted")
			}
		}

		receipt, err := rt.ExecuteTransaction(tx)
		if err != nil {
			return nil, nil, err
		}

		totalGasUsed += receipt.GasUsed
		receipts = append(receipts, receipt)
		processedTxs[tx.ID()] = receipt.Reverted
	}

	if header.GasUsed() != totalGasUsed {
		return nil, nil, consensusError(fmt.Sprintf("block gas used mismatch: want %v, have %v", header.GasUsed(), totalGasUsed))
	}

	receiptsRoot := receipts.RootHash()
	if header.ReceiptsRoot() != receiptsRoot {
		return nil, nil, consensusError(fmt.Sprintf("block receipts root mismatch: want %v, have %v", header.ReceiptsRoot(), receiptsRoot))
	}

	if err := rt.Seeker().Err(); err != nil {
		return nil, nil, errors.WithMessage(err, "chain")
	}

	stage := state.Stage()
	stateRoot, err := stage.Hash()
	if err != nil {
		return nil, nil, err
	}

	if blk.Header().StateRoot() != stateRoot {
		return nil, nil, consensusError(fmt.Sprintf("block state root mismatch: want %v, have %v", header.StateRoot(), stateRoot))
	}

	return stage, receipts, nil
}

//-----------------------------------------------------------
//---------------block store new wrappers routines ----------

//
func (conR *ConsensusReactor) finalizeMBlock(block *block.Block) bool {
	return true
}

func (conR *ConsensusReactor) finalizeKBlock(block *block.Block) bool {
	return true
}

func (conR *ConsensusReactor) finalizeCommitBlock(block *block.Block) bool {
	/*******************
	height := block.BlockHeight
	if (conR.curHeight + 1) != height {
		conR.Logger.Error(fmt.Sprintf("finalizeCommitBlock(%v): Invalid height. Current: %v/%v", height, conR.curHeight, conR.curRound))
		return false
	}

	//get block reactor
	bcR := blockchain.GetGlobBlockChainReactor()
	if bcR == nil {
		panic("Error getting block chain reactor ")
		return false
	}

	blockStore := bcR.GetBlockStore()
	fmt.Println("blockStore Height", blockStore.Height(), "commit height", height)
	if height <= blockStore.Height() {
		fmt.Println("Height mismatch. my height", height, "stored height", blockStore.Height())
		return false
	}

	// XXX: need to validate the block with newEvidence
	//if blockchain.ValidateBlock(block) == false {
	//	fmt.Println("Validate block failed ...")
	//	return false
	//}

	// set prevHash
	if height > 0 {
		prevBlock := blockStore.LoadBlock(height - 1)
		if prevBlock != nil {
			block.SetBlockPrevHash(prevBlock.GetHash())
		}
	}

	// get current block hash
	//blockHash := block.getHash()

	// save this block to persistence store
	blockParts := block.MakePartSet(types.BlockPartSizeBytes)
	blockStore.SaveBlock(block, blockParts)

	// block is saved. before broadcast out, update pool height to indicated I
	// already have this block.
	bcR.GetBlockPool().IncrPoolHeight()
	bcR.BroadcastBlock(block)

	// apply block???
	*******************/

	return true
}

//build block committee info part
func (conR *ConsensusReactor) BuildCommitteeInfoFromMember(cms []CommitteeMember) []block.CommitteeInfo {
	cis := []block.CommitteeInfo{}

	for _, cm := range cms {
		ci := block.NewCommitteeInfo(crypto.FromECDSAPub(&cm.PubKey), cm.VotingPower, cm.Accum, cm.NetAddr,
			conR.csCommon.system.PubKeyToBytes(cm.CSPubKey), cm.CSIndex)
		cis = append(cis, *ci)
	}
	return (cis)
}

//de-serialize the block committee info part
func (conR *ConsensusReactor) BuildCommitteeMemberFromInfo(cis []block.CommitteeInfo) []CommitteeMember {
	cms := []CommitteeMember{}
	for _, ci := range cis {
		cm := NewCommitteeMember()

		pubKey, err := crypto.UnmarshalPubkey(ci.PubKey)
		if err != nil {
			panic(err)
		}
		cm.PubKey = *pubKey
		cm.VotingPower = ci.VotingPower
		cm.NetAddr = ci.NetAddr

		CSPubKey, err := conR.csCommon.system.PubKeyFromBytes(ci.CSPubKey)
		if err != nil {
			panic(err)
		}
		cm.CSPubKey = CSPubKey
		cm.CSIndex = ci.CSIndex

		cms = append(cms, *cm)
	}
	return (cms)
}

//build block committee info part
func (conR *ConsensusReactor) MakeBlockCommitteeInfo(cms []CommitteeMember) []byte {
	cis := []block.CommitteeInfo{}

	for _, cm := range cms {
		ci := block.NewCommitteeInfo(crypto.FromECDSAPub(&cm.PubKey), cm.VotingPower, cm.Accum, cm.NetAddr,
			conR.csCommon.system.PubKeyToBytes(cm.CSPubKey), cm.CSIndex)
		cis = append(cis, *ci)
	}
	return (cdc.MustMarshalBinaryBare(&cis))
}

//de-serialize the block committee info part
func (conR *ConsensusReactor) DecodeBlockCommitteeInfo(ciBytes []byte) (cis []block.CommitteeInfo, err error) {
	err = cdc.UnmarshalBinaryBare(ciBytes, &cis)
	return
}
