package powpool

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/thor"

	"github.com/btcsuite/btcd/wire"
)

type PowBlockInfo struct {
	// Pow header part
	Version        uint32
	HashPrevBlock  thor.Bytes32
	HashMerkleRoot thor.Bytes32
	Timestamp      uint32
	NBits          uint32
	Nonce          uint32
	// End of pow header part

	HeaderHash  thor.Bytes32
	Beneficiary thor.Address
	PowHeight   uint32
	RewardCoef  int64

	// Raw block
	Raw []byte
}

func NewPowBlockInfoFromPosKBlock(posBlock *block.Block) *PowBlockInfo {
	if len(posBlock.KBlockData.Data) > 0 {
		data := posBlock.KBlockData.Data[0]
		powBlock := wire.MsgBlock{}
		powBlock.Deserialize(strings.NewReader(string(data)))
		info := NewPowBlockInfoFromPowBlock(&powBlock)

		buf := bytes.NewBufferString(string(data))
		buf.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
		posBlock.EncodeRLP(buf)

		info.Raw = buf.Bytes()
		return info
	}
	return nil
}

func GetPowGenesisBlockInfo() *PowBlockInfo {
	h := "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a29ab5f49ffff001d1dac2b7c0101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4d04ffff001d0104455468652054696d65732030332f4a616e2f32303039204368616e63656c6c6f72206f6e206272696e6b206f66207365636f6e64206261696c6f757420666f722062616e6b73ffffffff0100f2052a01000000434104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac00000000"
	b, _ := hex.DecodeString(h)
	powBlk := wire.MsgBlock{}
	powBlk.Deserialize(bytes.NewReader(b))
	powHdr := powBlk.Header

	prevBytes := reverse(powHdr.PrevBlock.CloneBytes())
	merkleRootBytes := reverse(powHdr.MerkleRoot.CloneBytes())

	info := &PowBlockInfo{
		Version:        uint32(powHdr.Version),
		HashPrevBlock:  thor.BytesToBytes32(prevBytes),
		HashMerkleRoot: thor.BytesToBytes32(merkleRootBytes),
		Timestamp:      uint32(powHdr.Timestamp.UnixNano()),
		NBits:          powHdr.Bits,
		Nonce:          powHdr.Nonce,

		Raw:        b,
		RewardCoef: POW_DEFAULT_REWARD_COEF,

		PowHeight: 0,
	}
	info.HeaderHash = info.HashID()
	return info
}

func NewPowBlockInfoFromPowBlock(powBlock *wire.MsgBlock) *PowBlockInfo {
	hdr := powBlock.Header
	prevBytes := reverse(hdr.PrevBlock.CloneBytes())
	merkleRootBytes := reverse(hdr.MerkleRoot.CloneBytes())

	buf := bytes.NewBufferString("")
	powBlock.Serialize(buf)

	var height uint32
	beneficiaryAddr := "0x"
	if len(powBlock.Transactions) == 1 && len(powBlock.Transactions[0].TxIn) == 1 {
		ss := powBlock.Transactions[0].TxIn[0].SignatureScript
		height, beneficiaryAddr = DecodeSignatureScript(ss)
	}
	beneficiary, _ := thor.ParseAddress(beneficiaryAddr)

	info := &PowBlockInfo{
		Version:        uint32(hdr.Version),
		HashPrevBlock:  thor.BytesToBytes32(prevBytes),
		HashMerkleRoot: thor.BytesToBytes32(merkleRootBytes),
		Timestamp:      uint32(hdr.Timestamp.UnixNano()),
		NBits:          hdr.Bits,
		Nonce:          hdr.Nonce,

		Raw:        buf.Bytes(),
		RewardCoef: POW_DEFAULT_REWARD_COEF,

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

func DecodeSignatureScript(bs []byte) (uint32, string) {
	items := make([][]byte, 0)

	for len(bs) > 0 {
		l := int(bs[0])
		end := 1 + l
		items = append(items, bs[1:end])
		bs = bs[end:]
	}

	var height uint32
	for _, b := range reverse(items[0]) {
		height = height*256 + uint32(b)
	}

	runes := make([]rune, 0)
	for _, b := range items[len(items)-1] {
		runes = append(runes, rune(b))
	}
	beneficiary := "0x" + string(runes)
	return height, beneficiary
}

func NewPowBlockInfo(raw []byte) *PowBlockInfo {
	blk := wire.MsgBlock{}
	blk.Deserialize(strings.NewReader(string(raw)))
	info := NewPowBlockInfoFromPowBlock(&blk)
	return info
}

func (info *PowBlockInfo) HashID() thor.Bytes32 {
	hash, _ := rlp.EncodeToBytes([]interface{}{
		info.Version,
		info.HashPrevBlock,
		info.HashMerkleRoot,
		info.Timestamp,
		info.NBits,
		info.Nonce,
	})
	return thor.Blake2b(hash)
}

func (info *PowBlockInfo) ToString() string {
	return fmt.Sprintf("&PowBlockInfo{\n\tVersion: 0x%x\n\tHashPrevBlock: %v\n\tHashMerkleRoot: %v\n\tNBits: 0x%x\n\tNonce: 0x%x \n\tHeaderHash: %v\n\tPowHeight: %v\n\tBeneficiary: %v\n}",
		info.Version, info.HashPrevBlock, info.HashMerkleRoot, info.NBits, info.Nonce, info.HeaderHash, info.PowHeight, info.Beneficiary)
}
