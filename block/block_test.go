// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	. "github.com/vechain/thor/block"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"

	"crypto/rand"
	cmn "github.com/vechain/thor/libs/common"
)

func TestBlock(t *testing.T) {

	tx1 := new(tx.Builder).Clause(tx.NewClause(&thor.Address{})).Clause(tx.NewClause(&thor.Address{})).Build()
	tx2 := new(tx.Builder).Clause(tx.NewClause(nil)).Build()

	privKey := string("dce1443bd2ef0c2631adc1c67e5c93f13dc23a41c18b536effbbdcbcdb96fb65")

	now := uint64(time.Now().UnixNano())

	var (
		gasUsed     uint64       = 1000
		gasLimit    uint64       = 14000
		totalScore  uint64       = 101
		emptyRoot   thor.Bytes32 = thor.BytesToBytes32([]byte("0"))
		beneficiary thor.Address = thor.BytesToAddress([]byte("abc"))
	)

	block := new(Builder).
		GasUsed(gasUsed).
		Transaction(tx1).
		Transaction(tx2).
		GasLimit(gasLimit).
		TotalScore(totalScore).
		StateRoot(emptyRoot).
		ReceiptsRoot(emptyRoot).
		Timestamp(now).
		ParentID(emptyRoot).
		Beneficiary(beneficiary).
		Build()

	h := block.Header()

	txs := block.Transactions()
	body := block.Body()
	txsRootHash := txs.RootHash()

	fmt.Println(h.ID())

	assert.Equal(t, body.Txs, txs)
	assert.Equal(t, Compose(h, txs), block)
	assert.Equal(t, gasLimit, h.GasLimit())
	assert.Equal(t, gasUsed, h.GasUsed())
	assert.Equal(t, totalScore, h.TotalScore())
	assert.Equal(t, emptyRoot, h.StateRoot())
	assert.Equal(t, emptyRoot, h.ReceiptsRoot())
	assert.Equal(t, now, h.Timestamp())
	assert.Equal(t, emptyRoot, h.ParentID())
	assert.Equal(t, beneficiary, h.Beneficiary())
	assert.Equal(t, txsRootHash, h.TxsRoot())

	key, _ := crypto.HexToECDSA(privKey)
	sig, _ := crypto.Sign(block.Header().SigningHash().Bytes(), key)

	block = block.WithSignature(sig)

	data, _ := rlp.EncodeToBytes(block)
	fmt.Println("DATA: ", data)
	fmt.Println("DATA STRING: ", string(data))

	b := Block{}

	rlp.DecodeBytes(data, &b)
	fmt.Println("BLOCK:", &b)
}

func TestBlockSerializeWithAmino(t *testing.T) {

	tx1 := new(tx.Builder).Clause(tx.NewClause(&thor.Address{})).Clause(tx.NewClause(&thor.Address{})).Build()
	tx2 := new(tx.Builder).Clause(tx.NewClause(nil)).Build()

	privKey := string("dce1443bd2ef0c2631adc1c67e5c93f13dc23a41c18b536effbbdcbcdb96fb65")

	now := uint64(time.Now().UnixNano())

	var (
		gasUsed     uint64       = 1000
		gasLimit    uint64       = 14000
		totalScore  uint64       = 101
		emptyRoot   thor.Bytes32 = thor.BytesToBytes32([]byte("0"))
		beneficiary thor.Address = thor.BytesToAddress([]byte("abc"))
	)

	block := new(Builder).
		GasUsed(gasUsed).
		Transaction(tx1).
		Transaction(tx2).
		GasLimit(gasLimit).
		TotalScore(totalScore).
		StateRoot(emptyRoot).
		ReceiptsRoot(emptyRoot).
		Timestamp(now).
		ParentID(emptyRoot).
		Beneficiary(beneficiary).
		Build()

	key, _ := crypto.HexToECDSA(privKey)
	sig, _ := crypto.Sign(block.Header().SigningHash().Bytes(), key)

	block = block.WithSignature(sig)

	var votingMsgHash, notarizeMsgHash [32]byte
	var votingBA, notarizeBA cmn.BitArray
	votingSig := make([]byte, 128)
	notarizeSig := make([]byte, 512)
	rand.Read(votingSig)
	rand.Read(notarizeSig)
	ev := NewEvidence(votingSig, votingMsgHash, votingBA, notarizeSig, notarizeMsgHash, notarizeBA)

	// func NewCommitteeInfo(pubKey []byte, power int64, accum int64, netAddr types.NetAddress, csPubKey []byte, csIndex int)

	block.SetBlockEvidence(ev)

	data := BlockEncodeBytes(block)
	fmt.Println("ENCODED BLOCK DATA: ", data)
	blk, err := BlockDecodeFromBytes(data)
	if err != nil {
		fmt.Println("ERROR: ", err)
	}
	fmt.Println("DECODED BLOCK: ", blk)
}
