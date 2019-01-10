// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	//"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/vechain/thor/block"

	//"github.com/vechain/thor/builtin"

	//"github.com/vechain/thor/chain"
	//"github.com/vechain/thor/poa"
	"github.com/vechain/thor/comm"
	"github.com/vechain/thor/runtime"
	"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
	"github.com/vechain/thor/txpool"

	//"github.com/vechain/thor/types"
	"github.com/ethereum/go-ethereum/common/mclock"
	crypto "github.com/ethereum/go-ethereum/crypto"
	bls "github.com/vechain/thor/crypto/multi_sig"
	"github.com/vechain/thor/packer"
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

	blk, err := c.chain.GetBlock(header.ID())
	if err != nil {
		return nil, err
	}

	if err := c.validateEvidence(blk.GetBlockEvidence(), blk); err != nil {
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

	if err := c.validateEvidence(block.GetBlockEvidence(), block); err != nil {
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

	// XXX: unlike vechain, our block generation is not periodically. It is on varied speed, basically 5s < t  < 15s
	/*****
	if (header.Timestamp()-parent.Timestamp())%thor.BlockInterval != 0 {
		return consensusError(fmt.Sprintf("block interval not rounded: parent %v, current %v", parent.Timestamp(), header.Timestamp()))
	}
	******/

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

func (c *ConsensusReactor) validateEvidence(ev *block.Evidence, blk *block.Block) error {
	// Yang: XXX: tmp only
	return nil
	header := blk.Header()

	// find out the block which has the committee info
	// Normally we store the committee info in the first of Mblock after Kblock
	lastKBlock := header.LastKBlockHeight()
	var b *block.Block
	if (lastKBlock + 1) == header.Number() {
		b = blk
	} else {
		var err error
		b, err = c.chain.GetTrunkBlock(lastKBlock + 1)
		if err != nil {
			fmt.Printf("get committee info block error")
			return consensusError(fmt.Sprintf("get committee info block failed: %v", err))
		}
	}
	c.logger.Info("get committeeinfo from block", b.Header().Number())

	// committee members
	cis, err := b.GetComitteeInfo()
	if err != nil {
		fmt.Printf("decode committee info block error")
		return consensusError(fmt.Sprintf("decode committee info block failed: %v", err))
	}

	cms := c.BuildCommitteeMemberFromInfo(cis)
	if len(cms) == 0 {
		fmt.Printf("get committee members error")
		return consensusError(fmt.Sprintf("get committee members failed: %v", err))
	}

	//validate voting signature
	voteSig, err := c.csCommon.system.SigFromBytes(ev.VotingSig)
	if err != nil {
		fmt.Printf("Sig from bytes error")
		return consensusError(fmt.Sprintf("voting signature from bytes failed: %v", err))
	}
	voteBA := ev.VotingBitArray

	for _, cm := range cms {
		if voteBA.GetIndex(cm.CSIndex) == true {
			if bls.Verify(voteSig, ev.VotingMsgHash, cm.CSPubKey) != true {
				fmt.Printf("voting signature validate error")
				return consensusError(fmt.Sprintf("voting signature validate error"))
			}
		}
	}

	//validate notarize signature
	notarizeSig, err := c.csCommon.system.SigFromBytes(ev.NotarizeSig)
	if err != nil {
		fmt.Printf("Notarize Sig from bytes error")
		return consensusError(fmt.Sprintf("notarize signature from bytes failed: %v", err))
	}
	notarizeBA := ev.NotarizeBitArray

	for _, cm := range cms {
		if notarizeBA.GetIndex(cm.CSIndex) == true {
			if bls.Verify(notarizeSig, ev.NotarizeMsgHash, cm.CSPubKey) != true {
				fmt.Printf("notarize signature validate error")
				return consensusError(fmt.Sprintf("notarize signature validate error"))
			}
		}
	}

	return nil
}

func (c *ConsensusReactor) validateProposer(header *block.Header, parent *block.Header, st *state.State) error {
	signer, err := header.Signer()
	if err != nil {
		return consensusError(fmt.Sprintf("block signer unavailable: %v", err))
	}
	fmt.Println("signer", signer)

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
func (conR *ConsensusReactor) finalizeMBlock(blk *block.Block, ev *block.Evidence) bool {

	var committeeInfo []block.CommitteeInfo
	if conR.curRound == 0 {
		committeeInfo = []block.CommitteeInfo{}
	} else {
		committeeInfo = conR.MakeBlockCommitteeInfo(conR.curActualCommittee)
	}

	blk.SetBlockEvidence(ev)
	blk.SetCommitteeInfo(committeeInfo)

	// XXX: update the cache size of this block. the
	return true
}

func (conR *ConsensusReactor) finalizeKBlock(blk *block.Block, ev *block.Evidence) bool {
	blk.SetBlockEvidence(ev)

	// XXX:update the cache size
	return true
}

func (conR *ConsensusReactor) finalizeCommitBlock(blkInfo *ProposedBlockInfo) bool {
	blk := blkInfo.ProposedBlock
	stage := blkInfo.Stage
	receipts := blkInfo.Receipts

	height := int64(blk.Header().Number())
	if (conR.curHeight + 1) != height {
		conR.logger.Error(fmt.Sprintf("finalizeCommitBlock(%v): Invalid height. curHeight:%v / curRound: %v", height, conR.curHeight, conR.curRound))
		return false
	}

	// similar to node.processBlock
	startTime := mclock.Now()

	if _, err := stage.Commit(); err != nil {
		conR.logger.Error("failed to commit state", "err", err)
		return false
	}

	fork, err := conR.chain.AddBlock(blk, *receipts)
	if err != nil {
		conR.logger.Error("add block failed ...", "err", err)
		return false
	}

	// unlike processBlock, we do not need to handle fork
	if fork != nil {
		//panic(" chain is in forked state, something wrong")
		//return false
		// process fork????
		if len(fork.Branch) > 0 {
			conR.logger.Warn("Fork Happened ...", "fork.Branch", len(fork.Branch))
			panic("Fork happened!")
		}
	}

	commitElapsed := mclock.Now() - startTime

	// XXX: broadcast the new block to all peers
	comm.GetGlobCommInst().BroadcastBlock(blk)
	// successfully added the block, update the current hight of consensus
	conR.logger.Info(fmt.Sprintf(`
===========================================================
Block commited at height %d
===========================================================
`, height), "elapsedTime", commitElapsed, "bestBlockHeight", conR.chain.BestBlock().Header().Number())
	fmt.Println(blk)
	conR.UpdateHeight(int64(conR.chain.BestBlock().Header().Number()))

	// Update lastKBlockHeight if necessary
	if blk.Header().BlockType() == block.BLOCK_TYPE_K_BLOCK {
		conR.lastKBlockHeight = blk.Header().Number()
	}

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
func (conR *ConsensusReactor) MakeBlockCommitteeInfo(cms []CommitteeMember) []block.CommitteeInfo {
	cis := []block.CommitteeInfo{}

	for _, cm := range cms {
		ci := block.NewCommitteeInfo(crypto.FromECDSAPub(&cm.PubKey), cm.VotingPower, cm.Accum, cm.NetAddr,
			conR.csCommon.system.PubKeyToBytes(cm.CSPubKey), cm.CSIndex)
		cis = append(cis, *ci)
	}
	return (cis)
}

// MBlock Routine
//=================================================

// proposed block info
type ProposedBlockInfo struct {
	ProposedBlock *block.Block
	Stage         *state.Stage
	Receipts      *tx.Receipts
}

// Build MBlock
func (conR *ConsensusReactor) BuildMBlock() *ProposedBlockInfo {
	best := conR.chain.BestBlock()
	now := uint64(time.Now().Unix())
	if conR.curHeight != int64(best.Header().Number()) {
		conR.logger.Error("Proposed block parent is not current best block")
		return nil
	}

	startTime := mclock.Now()
	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		conR.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}

	txs := pool.Executables()
	var txsToRemove []thor.Bytes32
	defer func() {
		for _, id := range txsToRemove {
			pool.Remove(id)
		}
	}()

	p := packer.GetGlobPackerInst()
	if p == nil {
		conR.logger.Error("get packer failed ...")
		panic("get packer failed")
		return nil
	}

	gasLimit := p.GasLimit(best.Header().GasLimit())
	flow, err := p.Mock(best.Header(), now, gasLimit)
	if err != nil {
		conR.logger.Error("mock packer", "error", err)
		return nil
	}

	for _, tx := range txs {
		if err := flow.Adopt(tx); err != nil {
			if packer.IsGasLimitReached(err) {
				break
			}
			if packer.IsTxNotAdoptableNow(err) {
				continue
			}
			txsToRemove = append(txsToRemove, tx.ID())
		}
	}

	newBlock, stage, receipts, err := flow.Pack(&conR.myPrivKey, block.BLOCK_TYPE_M_BLOCK, conR.lastKBlockHeight)
	if err != nil {
		conR.logger.Error("build block failed")
		return nil
	}

	execElapsed := mclock.Now() - startTime
	conR.logger.Info("MBlock built", "height", conR.curHeight, "elapseTime", execElapsed)
	return &ProposedBlockInfo{newBlock, stage, &receipts}
}

func (conR *ConsensusReactor) BuildKBlock(data *block.KBlockData) *ProposedBlockInfo {
	best := conR.chain.BestBlock()
	now := uint64(time.Now().Unix())
	if conR.curHeight != int64(best.Header().Number()) {
		conR.logger.Info("Proposed block parent is not current best block")
		return nil
	}

	startTime := mclock.Now()
	//XXX: Build kblock coinbse Tranactions
	txs := tx.Transactions{}

	p := packer.GetGlobPackerInst()
	if p == nil {
		conR.logger.Warn("get packer failed ...")
		panic("get packer failed")
		return nil
	}

	var txsToRemove []thor.Bytes32
	gasLimit := p.GasLimit(best.Header().GasLimit())
	flow, err := p.Mock(best.Header(), now, gasLimit)
	if err != nil {
		conR.logger.Warn("mock packer", "error", err)
		return nil
	}

	for _, tx := range txs {
		if err := flow.Adopt(tx); err != nil {
			if packer.IsGasLimitReached(err) {
				break
			}
			if packer.IsTxNotAdoptableNow(err) {
				continue
			}
			txsToRemove = append(txsToRemove, tx.ID())
		}
	}

	newBlock, stage, receipts, err := flow.Pack(&conR.myPrivKey, block.BLOCK_TYPE_K_BLOCK, conR.lastKBlockHeight)
	if err != nil {
		conR.logger.Error("build block failed")
		return nil
	}

	//serialize KBlockData
	newBlock.SetKBlockData(data)

	execElapsed := mclock.Now() - startTime
	conR.logger.Info("KBlock built", "height", conR.curHeight, "elapseTime", execElapsed)
	return &ProposedBlockInfo{newBlock, stage, &receipts}
}

//=================================================
// handle KBlock info info message from node
const (
	genesisNonce = uint64(1001)
)

type RecvKBlockInfo struct {
	Height           int64
	LastKBlockHeight uint32
	Nonce            uint64
}

func (conR *ConsensusReactor) HandleRecvKBlockInfo(ki RecvKBlockInfo) error {
	best := conR.chain.BestBlock()

	if ki.Height != int64(best.Header().Number()) {
		conR.logger.Info("kblock info height ", ki.Height, "my best", best.Header().Number(), "is ignored")
		return nil
	}

	if best.Header().BlockType() != block.BLOCK_TYPE_K_BLOCK {
		conR.logger.Info("best block is not kblock")
		return nil
	}
	// Now handle this nonce. Exit the committee if it is still in.
	conR.exitCurCommittee()

	// run new one.
	conR.ConsensusHandleReceivedNonce(ki.Height, ki.Nonce)
	return nil
}

func (conR *ConsensusReactor) HandleKBlockData(kd block.KBlockData) {
	conR.kBlockData = &kd
}