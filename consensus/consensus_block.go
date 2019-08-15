// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	//"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/dfinlab/meter/block"
	"github.com/pkg/errors"

	//"github.com/dfinlab/meter/builtin"

	//"github.com/dfinlab/meter/chain"
	//"github.com/dfinlab/meter/poa"
	"github.com/dfinlab/meter/comm"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/runtime"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/txpool"

	//"github.com/dfinlab/meter/types"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
	"github.com/dfinlab/meter/logdb"
	"github.com/dfinlab/meter/packer"
	"github.com/dfinlab/meter/xenv"
	"github.com/ethereum/go-ethereum/common/mclock"
	crypto "github.com/ethereum/go-ethereum/crypto"
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

func (c *ConsensusReactor) ProcessProposedBlock(parentHeader *block.Header, blk *block.Block, nowTimestamp uint64) (*state.Stage, tx.Receipts, error) {
	header := blk.Header()

	if _, err := c.chain.GetBlockHeader(header.ID()); err != nil {
		if !c.chain.IsNotFound(err) {
			return nil, nil, err
		}
	} else {
		return nil, nil, errKnownBlock
	}

	if parentHeader == nil {
		return nil, nil, errParentHeaderMissing
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

	//TBD: hotstuff, current block's evidence is not available yet. comment out
	// we can validate this evidence with block parent.
	/******
	blk, err := c.chain.GetBlock(header.ID())
	if err != nil {
		return nil, err
	}

	if err := c.validateEvidence(blk.GetBlockEvidence(), blk); err != nil {
		return nil, err
	}
	****/
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

	//same above reason.
	/****
	if err := c.validateEvidence(block.GetBlockEvidence(), block); err != nil {
		return nil, nil, err
	}
	****/
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
	if (header.Timestamp()-parent.Timestamp())%meter.BlockInterval != 0 {
		return consensusError(fmt.Sprintf("block interval not rounded: parent %v, current %v", parent.Timestamp(), header.Timestamp()))
	}
	******/

	if header.Timestamp() > nowTimestamp+meter.BlockInterval {
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
	// skip the validation if the 'skip-signature-check' flag is configured
	if c.config.SkipSignatureCheck {
		return nil
	}

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
			c.logger.Error("get committee info block error")
			return consensusError(fmt.Sprintf("get committee info block failed: %v", err))
		}
	}
	c.logger.Info("get committeeinfo from block", "height", b.Header().Number())

	// re-init the BLS systerm
	systemBytes, _ := b.GetSystemBytes()
	paramsBytes, _ := b.GetParamsBytes()
	system, params, pairing, err := c.CreateCSSystem(systemBytes, paramsBytes)
	if err != nil {
		c.logger.Error("BLS system init error")
		return consensusError(fmt.Sprintf("BLS system init failed: %v", err))
	}

	// committee members
	cis, err := b.GetCommitteeInfo()
	if err != nil {
		c.logger.Error("decode committee info block error")
		c.FreeCSSystem(system, params, pairing)
		return consensusError(fmt.Sprintf("decode committee info block failed: %v", err))
	}

	cms := c.BuildCommitteeMemberFromInfo(system, cis)
	if len(cms) == 0 {
		c.logger.Error("get committee members error")
		c.FreeCSSystem(system, params, pairing)
		return consensusError(fmt.Sprintf("get committee members failed: %v", err))
	}

	//validate voting signature
	voteSig, err := system.SigFromBytes(ev.VotingSig)
	if err != nil {
		c.logger.Error("Sig from bytes error")
		c.FreeCSSystem(system, params, pairing)
		return consensusError(fmt.Sprintf("voting signature from bytes failed: %v", err))
	}

	voteBA := ev.VotingBitArray
	voteCSPubKeys := []bls.PublicKey{}
	for _, cm := range cms {
		if voteBA.GetIndex(cm.CSIndex) == true {
			voteCSPubKeys = append(voteCSPubKeys, cm.CSPubKey)
		}
	}

	/*****
	fmt.Println("VoterMsgHash", cmn.ByteSliceToByte32(ev.VotingMsgHash))
	for _, p := range voteCSPubKeys {
		fmt.Println("pubkey:::", system.PubKeyToBytes(p))
	}
	fmt.Println("aggrsig", system.SigToBytes(voteSig), "system", system.ToBytes())
	*****/

	voteValid, err := bls.AggregateVerify(voteSig, cmn.ByteSliceToByte32(ev.VotingMsgHash), voteCSPubKeys)
	if (err != nil) || (voteValid != true) {
		c.logger.Error("voting signature validate error")
		c.FreeCSSystem(system, params, pairing)
		return consensusError(fmt.Sprintf("voting signature validate error"))
	}

	//validate notarize signature
	notarizeSig, err := system.SigFromBytes(ev.NotarizeSig)
	if err != nil {
		c.logger.Error("Notarize Sig from bytes error")
		c.FreeCSSystem(system, params, pairing)
		return consensusError(fmt.Sprintf("notarize signature from bytes failed: %v", err))
	}
	notarizeBA := ev.NotarizeBitArray
	notarizeCSPubKeys := []bls.PublicKey{}

	for _, cm := range cms {
		if notarizeBA.GetIndex(cm.CSIndex) == true {
			notarizeCSPubKeys = append(notarizeCSPubKeys, cm.CSPubKey)
		}
	}
	noterizeValid, err := bls.AggregateVerify(notarizeSig, cmn.ByteSliceToByte32(ev.NotarizeMsgHash), notarizeCSPubKeys)
	if (err != nil) || (noterizeValid != true) {
		c.logger.Error("notarize signature validate error")
		c.FreeCSSystem(system, params, pairing)
		return consensusError(fmt.Sprintf("notarize signature validate error"))
	}

	c.FreeCSSystem(system, params, pairing)
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

	for i, tx := range txs {
		signer, err := tx.Signer()
		if err != nil {
			return consensusError(fmt.Sprintf("tx signer unavailable: %v", err))
		}

		// Mint transaction critiers:
		// 1. no signature (no signer)
		// 2. only located in 1st transaction in kblock.
		if signer.IsZero() {
			if (i != 0) || (blk.Header().BlockType() != block.BLOCK_TYPE_K_BLOCK) {
				return consensusError(fmt.Sprintf("tx signer unavailable"))
			}
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
	processedTxs := make(map[meter.Bytes32]bool)
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

	findTx := func(txID meter.Bytes32) (found bool, reverted bool, err error) {
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

	for i, tx := range txs {
		// Mint transaction critiers:
		// 1. no signature (no signer)
		// 2. only located in 1st transaction in kblock.
		signer, err := tx.Signer()
		if err != nil {
			return nil, nil, consensusError(fmt.Sprintf("tx signer unavailable: %v", err))
		}

		if signer.IsZero() {
			if (i != 0) || (blk.Header().BlockType() != block.BLOCK_TYPE_K_BLOCK) {
				return nil, nil, consensusError(fmt.Sprintf("tx signer unavailable"))
			}
		}

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
	if conR.curRound != 0 {
		committeeInfo = []block.CommitteeInfo{}
	} else {
		// only round 0 Mblock contains the following info
		committeeInfo = conR.MakeBlockCommitteeInfo(conR.csCommon.system, conR.curActualCommittee)
		systemBytes := conR.csCommon.system.ToBytes()
		blk.SetSystemBytes(systemBytes)

		paramsBytes, _ := conR.csCommon.params.ToBytes()
		blk.SetParamsBytes(paramsBytes)

		blk.SetCommitteeEpoch(conR.curEpoch)
	}

	blk.SetBlockEvidence(ev)
	blk.SetCommitteeInfo(committeeInfo)

	//Fill new info into block, re-calc hash/signature
	blk.SetEvidenceDataHash(blk.EvidenceDataHash())
	sig, err := crypto.Sign(blk.Header().SigningHash().Bytes(), &conR.myPrivKey)
	if err != nil {
		return false
	}

	blk.SetBlockSignature(sig)
	return true
}

func (conR *ConsensusReactor) finalizeKBlock(blk *block.Block, ev *block.Evidence) bool {
	blk.SetBlockEvidence(ev)

	// XXX:update the cache size

	// only round 0 Mblock contains the following info
	committeeInfo := conR.MakeBlockCommitteeInfo(conR.csCommon.system, conR.curActualCommittee)
	systemBytes := conR.csCommon.system.ToBytes()
	blk.SetSystemBytes(systemBytes)

	paramsBytes, _ := conR.csCommon.params.ToBytes()
	blk.SetParamsBytes(paramsBytes)
	blk.SetCommitteeInfo(committeeInfo)
	blk.SetCommitteeEpoch(conR.curEpoch)

	//Fill new info into block, re-calc hash/signature
	blk.SetEvidenceDataHash(blk.EvidenceDataHash())
	sig, err := crypto.Sign(blk.Header().SigningHash().Bytes(), &conR.myPrivKey)
	if err != nil {
		return false
	}

	blk.SetBlockSignature(sig)
	return true
}

func (conR *ConsensusReactor) finalizeCommitBlock(blkInfo *ProposedBlockInfo) bool {
	blk := blkInfo.ProposedBlock
	stage := blkInfo.Stage
	receipts := blkInfo.Receipts

	height := uint64(blk.Header().Number())

	// TODO: temporary remove
	// if conR.csPacemaker.blockLocked.Height != height+1 {
	// conR.logger.Error(fmt.Sprintf("finalizeCommitBlock(H:%v): Invalid height. bLocked Height:%v, curRround: %v", height, conR.csPacemaker.blockLocked.Height, conR.curRound))
	// return false
	// }
	conR.logger.Debug("Try to commit block: ", "block", blk.Oneliner())

	// similar to node.processBlock
	startTime := mclock.Now()

	if _, err := stage.Commit(); err != nil {
		conR.logger.Error("failed to commit state", "err", err)
		return false
	}

	batch := logdb.GetGlobalLogDBInstance().Prepare(blk.Header())
	for i, tx := range blk.Transactions() {
		origin, _ := tx.Signer()
		txBatch := batch.ForTransaction(tx.ID(), origin)
		for _, output := range (*(*receipts)[i]).Outputs {
			txBatch.Insert(output.Events, output.Transfers)
		}
	}

	if err := batch.Commit(); err != nil {
		conR.logger.Error("commit logs failed ...", "err", err)
		return false
	}

	fork, err := conR.chain.AddBlock(blk, *receipts, true)
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

	// now only Mblock remove the txs from txpool
	blkInfo.txsToRemoved()

	commitElapsed := mclock.Now() - startTime

	blocksCommitedCounter.Inc()

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

	return true
}

//build block committee info part
func (conR *ConsensusReactor) BuildCommitteeInfoFromMember(system bls.System, cms []CommitteeMember) []block.CommitteeInfo {
	cis := []block.CommitteeInfo{}

	for _, cm := range cms {
		ci := block.NewCommitteeInfo(crypto.FromECDSAPub(&cm.PubKey), uint64(cm.VotingPower), cm.NetAddr,
			system.PubKeyToBytes(cm.CSPubKey), uint32(cm.CSIndex))
		cis = append(cis, *ci)
	}
	return (cis)
}

//de-serialize the block committee info part
func (conR *ConsensusReactor) BuildCommitteeMemberFromInfo(system bls.System, cis []block.CommitteeInfo) []CommitteeMember {
	cms := []CommitteeMember{}
	for _, ci := range cis {
		cm := NewCommitteeMember()

		pubKey, err := crypto.UnmarshalPubkey(ci.PubKey)
		if err != nil {
			panic(err)
		}
		cm.PubKey = *pubKey
		cm.VotingPower = int64(ci.VotingPower)
		cm.NetAddr = ci.NetAddr
		cm.Address = meter.Address(crypto.PubkeyToAddress(*pubKey))

		CSPubKey, err := system.PubKeyFromBytes(ci.CSPubKey)
		if err != nil {
			panic(err)
		}
		cm.CSPubKey = CSPubKey
		cm.CSIndex = int(ci.CSIndex)

		cms = append(cms, *cm)
	}
	return (cms)
}

//build block committee info part
func (conR *ConsensusReactor) MakeBlockCommitteeInfo(system bls.System, cms []CommitteeMember) []block.CommitteeInfo {
	cis := []block.CommitteeInfo{}

	for _, cm := range cms {
		ci := block.NewCommitteeInfo(crypto.FromECDSAPub(&cm.PubKey), uint64(cm.VotingPower), cm.NetAddr,
			system.PubKeyToBytes(cm.CSPubKey), uint32(cm.CSIndex))
		cis = append(cis, *ci)
	}
	return (cis)
}

// A committee system are stored in evidence, need to re-init that system and free it after it is done.
// similar to the validator common init/deinit.
func (conR *ConsensusReactor) CreateCSSystem(systemBytes, paramBytes []byte) (bls.System, bls.Params, bls.Pairing, error) {
	params, err := bls.ParamsFromBytes(paramBytes)
	if err != nil {
		conR.logger.Error("initialize param failed...")
		panic(err)
	}

	pairing := bls.GenPairing(params)
	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		conR.logger.Error("initialize system failed...")
		panic(err)
	}

	return system, params, pairing, nil
}

func (conR *ConsensusReactor) FreeCSSystem(system bls.System, params bls.Params, pairing bls.Pairing) {
	system.Free()
	pairing.Free()
	params.Free()
}

// MBlock Routine
//=================================================

type BlockType int

const (
	KBlockType BlockType = 1
	MBlockType BlockType = 2
)

// proposed block info
type ProposedBlockInfo struct {
	ProposedBlock *block.Block
	Stage         *state.Stage
	Receipts      *tx.Receipts
	txsToRemoved  func() bool
	CheckPoint    int
	BlockType     BlockType
}

func (pb *ProposedBlockInfo) String() string {
	if pb == nil {
		return "ProposedBlockInfo(nil)"
	}
	if pb.ProposedBlock != nil {
		return "ProposedBlockInfo[block: " + pb.ProposedBlock.String() + "]"
	} else {
		return "ProposedBlockInfo[block: nil]"
	}
}

// Build MBlock
func (conR *ConsensusReactor) BuildMBlock(parentBlock *block.Block) *ProposedBlockInfo {
	best := parentBlock
	now := uint64(time.Now().Unix())
	/*
		TODO: better check this, comment out temporarily
		if conR.curHeight != int64(best.Header().Number()) {
			conR.logger.Error("Proposed block parent is not current best block")
			return nil
		}
	*/

	startTime := mclock.Now()
	pool := txpool.GetGlobTxPoolInst()
	if pool == nil {
		conR.logger.Error("get tx pool failed ...")
		panic("get tx pool failed ...")
		return nil
	}

	txs := pool.Executables()
	var txsToRemove []meter.Bytes32
	txsToRemoved := func() bool {
		for _, id := range txsToRemove {
			pool.Remove(id)
		}
		return true
	}

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

	//create checkPoint before build block
	state, err := conR.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		conR.logger.Error("revert state failed ...", "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

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
		conR.logger.Error("build block failed", "error", err)
		return nil
	}

	execElapsed := mclock.Now() - startTime
	conR.logger.Info("MBlock built", "height", conR.curHeight, "elapseTime", execElapsed)
	return &ProposedBlockInfo{newBlock, stage, &receipts, txsToRemoved, checkPoint, MBlockType}
}

func (conR *ConsensusReactor) BuildKBlock(parentBlock *block.Block, data *block.KBlockData, rewards []powpool.PowReward) *ProposedBlockInfo {
	best := parentBlock
	now := uint64(time.Now().Unix())
	/*
		TODO: better check this, comment out temporarily
		if conR.curHeight != int64(best.Header().Number()) {
			conR.logger.Warn("Proposed block parent is not current best block")
			return nil
		}
	*/

	conR.logger.Info("build kblock ...", "nonce", data.Nonce)
	startTime := mclock.Now()
	//XXX: Build kblock coinbase Tranactions
	//txs := tx.Transactions{}
	txs := conR.GetKBlockRewardTxs(rewards)

	p := packer.GetGlobPackerInst()
	if p == nil {
		conR.logger.Warn("get packer failed ...")
		panic("get packer failed")
		return nil
	}

	//var txsToRemove []meter.Bytes32
	txsToRemoved := func() bool {
		// Kblock does not need to clean up txs now
		return true
	}

	gasLimit := p.GasLimit(best.Header().GasLimit())
	flow, err := p.Mock(best.Header(), now, gasLimit)
	if err != nil {
		conR.logger.Warn("mock packer", "error", err)
		return nil
	}

	//create checkPoint before build block
	state, err := conR.stateCreator.NewState(best.Header().StateRoot())
	if err != nil {
		conR.logger.Error("revert state failed ...", "error", err)
		return nil
	}
	checkPoint := state.NewCheckpoint()

	for _, tx := range txs {
		if err := flow.Adopt(tx); err != nil {
			if packer.IsGasLimitReached(err) {
				break
			}
			if packer.IsTxNotAdoptableNow(err) {
				continue
			}
			//txsToRemove = append(txsToRemove, tx.ID())
		}
	}

	newBlock, stage, receipts, err := flow.Pack(&conR.myPrivKey, block.BLOCK_TYPE_K_BLOCK, conR.lastKBlockHeight)
	if err != nil {
		conR.logger.Error("build block failed...", "error", err)
		return nil
	}

	//serialize KBlockData
	newBlock.SetKBlockData(*data)

	execElapsed := mclock.Now() - startTime
	conR.logger.Info("KBlock built", "height", conR.curHeight, "elapseTime", execElapsed)
	return &ProposedBlockInfo{newBlock, stage, &receipts, txsToRemoved, checkPoint, KBlockType}
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
		conR.logger.Info("kblock info is ignored ...", "received hight", ki.Height, "my best", best.Header().Number())
		return nil
	}

	if best.Header().BlockType() != block.BLOCK_TYPE_K_BLOCK {
		conR.logger.Info("best block is not kblock")
		return nil
	}

	conR.logger.Info("received KBlock ...", "height", ki.Height, "lastKBlockHeight", ki.LastKBlockHeight, "nonce", ki.Nonce)

	// Now handle this nonce. Exit the committee if it is still in.
	conR.exitCurCommittee()

	// update last kblock height with current Height sine kblock is handled
	conR.UpdateLastKBlockHeight(best.Header().Number())

	// run new one.
	conR.ConsensusHandleReceivedNonce(ki.Height, ki.Nonce, false)
	return nil
}

func (conR *ConsensusReactor) HandleKBlockData(kd block.KBlockData) {
	conR.kBlockData = &kd
}

//========================================================
func (conR *ConsensusReactor) PreCommitBlock(blkInfo *ProposedBlockInfo) bool {
	blk := blkInfo.ProposedBlock
	stage := blkInfo.Stage
	receipts := blkInfo.Receipts

	height := uint64(blk.Header().Number())

	// TODO: temporary remove
	// if conR.csPacemaker.blockLocked.Height != height+1 {
	// conR.logger.Error(fmt.Sprintf("finalizeCommitBlock(H:%v): Invalid height. bLocked Height:%v, curRround: %v", height, conR.csPacemaker.blockLocked.Height, conR.curRound))
	// return false
	// }
	conR.logger.Debug("Try to pre-commit block", "block", blk.Oneliner())

	// similar to node.processBlock
	startTime := mclock.Now()

	if _, err := stage.Commit(); err != nil {
		conR.logger.Error("failed to commit state", "err", err)
		return false
	}

	/*****
		batch := logdb.GetGlobalLogDBInstance().Prepare(blk.Header())
		for i, tx := range blk.Transactions() {
			origin, _ := tx.Signer()
			txBatch := batch.ForTransaction(tx.ID(), origin)
			for _, output := range (*(*receipts)[i]).Outputs {
				txBatch.Insert(output.Events, output.Transfers)
			}
		}

		if err := batch.Commit(); err != nil {
			conR.logger.Error("commit logs failed ...", "err", err)
			return false
		}
	******/
	fork, err := conR.chain.AddBlock(blk, *receipts, false)
	if err != nil {
		if err == errKnownBlock {
			conR.logger.Warn("known block", "err", err)
		} else {
			conR.logger.Warn("add block failed ...", "err", err)
		}
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

	// now only Mblock remove the txs from txpool
	blkInfo.txsToRemoved()

	commitElapsed := mclock.Now() - startTime

	blocksCommitedCounter.Inc()
	conR.logger.Info(fmt.Sprintf(`
Block precommited at height %d
%v`, height, blk.CompactString()), "elapsedTime", commitElapsed, "bestBlockHeight", conR.chain.BestBlock().Header().Number())
	/*****
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
	  *******/
	return true
}

func (conR *ConsensusReactor) finalizeCommitBlock222(blkInfo *ProposedBlockInfo) bool {
	blk := blkInfo.ProposedBlock
	//stage := blkInfo.Stage
	receipts := blkInfo.Receipts

	height := uint64(blk.Header().Number())

	// TODO: temporary remove
	// if conR.csPacemaker.blockLocked.Height != height+1 {
	// conR.logger.Error(fmt.Sprintf("finalizeCommitBlock(H:%v): Invalid height. bLocked Height:%v, curRround: %v", height, conR.csPacemaker.blockLocked.Height, conR.curRound))
	// return false
	// }
	conR.logger.Debug("Try to finalize block", "block", blk.Oneliner())

	/***********************
			// similar to node.processBlock
			startTime := mclock.Now()

			if _, err := stage.Commit(); err != nil {
				conR.logger.Error("failed to commit state", "err", err)
				return false
			}

			batch := logdb.GetGlobalLogDBInstance().Prepare(blk.Header())
			for i, tx := range blk.Transactions() {
				origin, _ := tx.Signer()
				txBatch := batch.ForTransaction(tx.ID(), origin)
				for _, output := range (*(*receipts)[i]).Outputs {
					txBatch.Insert(output.Events, output.Transfers)
				}
			}

			if err := batch.Commit(); err != nil {
				conR.logger.Error("commit logs failed ...", "err", err)
				return false
			}
	*****/

	fork, err := conR.chain.AddBlock(blk, *receipts, true)
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
	/*****k

		// now only Mblock remove the txs from txpool
		blkInfo.txsToRemoved()

		commitElapsed := mclock.Now() - startTime

		blocksCommitedCounter.Inc()
	******/
	// XXX: broadcast the new block to all peers
	comm.GetGlobCommInst().BroadcastBlock(blk)
	// successfully added the block, update the current hight of consensus
	conR.logger.Info(fmt.Sprintf(`
-----------------------------------------------------------
Block commited at height %d
-----------------------------------------------------------
%v
`, height, blk.String()), /***"elapsedTime", commitElapsed,***/ "bestBlockHeight", conR.chain.BestBlock().Header().Number())
	conR.UpdateHeight(int64(conR.chain.BestBlock().Header().Number()))

	return true
}
