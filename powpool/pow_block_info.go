// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	// "github.com/ethereum/go-ethereum/rlp"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type PowBlockInfo struct {
	// Pow header part
	Version        uint32
	HashPrevBlock  meter.Bytes32
	HashMerkleRoot meter.Bytes32
	Timestamp      uint32
	NBits          uint32
	Nonce          uint32
	// End of pow header part

	HeaderHash  meter.Bytes32
	Beneficiary meter.Address
	PowHeight   uint32

	// Raw block
	PosRaw []byte
	PowRaw []byte
}

func NewPowBlockInfoFromPosKBlock(posBlock *block.Block) *PowBlockInfo {
	if len(posBlock.KBlockData.Data) > 0 {
		data := posBlock.KBlockData.Data[0]
		powBlock := wire.MsgBlock{}
		err := powBlock.Deserialize(strings.NewReader(string(data)))
		if err != nil {
			fmt.Println("could not deserialize msgBlock, error:", err)
		}

		info := NewPowBlockInfoFromPowBlock(&powBlock)

		buf := bytes.NewBufferString("")
		posBlock.EncodeRLP(buf)

		info.PowRaw = data
		info.PosRaw = buf.Bytes()
		return info
	}
	return nil
}

func GetPowGenesisBlockInfo() *PowBlockInfo {
	h := "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a29ab5f49ffff001d1dac2b7c0101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4d04ffff001d0104455468652054696d65732030332f4a616e2f32303039204368616e63656c6c6f72206f6e206272696e6b206f66207365636f6e64206261696c6f757420666f722062616e6b73ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000"
	b, _ := hex.DecodeString(h)
	powBlk := wire.MsgBlock{}
	powBlk.Deserialize(bytes.NewReader(b))
	info := NewPowBlockInfoFromPowBlock(&powBlk)
	info.PowHeight = 0
	return info
}

func NewPowBlockInfoFromPowBlock(powBlock *wire.MsgBlock) *PowBlockInfo {
	hdr := powBlock.Header
	prevBytes := reverse(hdr.PrevBlock.CloneBytes())
	merkleRootBytes := reverse(hdr.MerkleRoot.CloneBytes())

	buf := bytes.NewBufferString("")
	err := powBlock.Serialize(buf)
	if err != nil {
		fmt.Println("could not serialize msgBlock, error:", err)
	}

	var height uint32
	beneficiaryAddr := "0x"
	if len(powBlock.Transactions) == 1 && len(powBlock.Transactions[0].TxIn) == 1 {
		ss := powBlock.Transactions[0].TxIn[0].SignatureScript
		height, beneficiaryAddr = DecodeSignatureScript(ss)
	}
	beneficiary, _ := meter.ParseAddress(beneficiaryAddr)

	info := &PowBlockInfo{
		Version:        uint32(hdr.Version),
		HashPrevBlock:  meter.BytesToBytes32(prevBytes),
		HashMerkleRoot: meter.BytesToBytes32(merkleRootBytes),
		Timestamp:      uint32(hdr.Timestamp.UnixNano()),
		NBits:          hdr.Bits,
		Nonce:          hdr.Nonce,

		PowRaw: buf.Bytes(),

		PowHeight:   height,
		Beneficiary: beneficiary,
	}

	info.HeaderHash = info.HashID()
	return info
}

func reverse(a []byte) []byte {
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
	return a
}

// scriptSig contains 4 segments:
// encoded Height (vaired length) + Seqence (4 bytes) + TimeStamp (8 bytes) + Beneficiary str(40 bytes) + Optional（pool tag）
// only returns height， beneficairy as items[0],[3]
func DecodeSignatureScript(bs []byte) (uint32, string) {
	items := make([][]byte, 0)

	for len(bs) > 0 {
		l := int(bs[0])
		end := 1 + l
		items = append(items, bs[1:end])
		bs = bs[end:]
	}

	//fmt.Println("The items", items)
	if len(items) < 4 || len(items[1]) != 4 || len(items[2]) != 8 || len(items[3]) != 40 {
		// unrecognized script, the pow genesis does not follow this rule.
		return 0, "0x00000000000000000000"
	}

	var height uint32
	for _, b := range reverse(items[0]) {
		height = height*256 + uint32(b)
	}

	runes := make([]rune, 0)

	for _, b := range items[3] {
		runes = append(runes, rune(b))
	}
	beneficiary := "0x" + string(runes)
	return height, beneficiary
}

func NewPowBlockInfo(raw []byte) *PowBlockInfo {
	blk := wire.MsgBlock{}
	err := blk.Deserialize(strings.NewReader(string(raw)))
	if err != nil {
		fmt.Println("could not deserialize msgBlock, error:", err)
	}

	info := NewPowBlockInfoFromPowBlock(&blk)
	return info
}

func (info *PowBlockInfo) HashID() meter.Bytes32 {
	powBlk := wire.MsgBlock{}
	err := powBlk.Deserialize(bytes.NewReader(info.PowRaw))
	if err != nil {
		fmt.Println("could not deserialize msgBlock, error:", err)
	}

	var powBlkPtr *wire.MsgBlock
	powBlkPtr = &powBlk
	hash := powBlkPtr.BlockHash()
	var hashPtr *chainhash.Hash
	hashPtr = &hash
	bs := hashPtr.CloneBytes()
	var bss []byte
	bss = bs

	return meter.BytesToBytes32(reverse(bss))
}

func (info *PowBlockInfo) ToString() string {
	return fmt.Sprintf("PowBlockInfo{\n  PowHeight: %v, Version: 0x%x, NBits: 0x%x, Nonce: 0x%x\n  HeaderHash: %v\n  HashPrevBlock: %v\n  HashMerkleRoot: %v\n  Beneficiary: %v\n}",
		info.PowHeight, info.Version, info.NBits, info.Nonce, info.HeaderHash, info.HashPrevBlock, info.HashMerkleRoot, info.Beneficiary)
}
