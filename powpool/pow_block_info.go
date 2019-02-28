package powpool

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/thor"

	"github.com/btcsuite/btcd/wire"
)

var (
	powBlockVersion       = uint32(0x20000000)
	powKframeBlockVersion = uint32(0x20100000)
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
	RewardCoef  uint64

	// Raw block
	Raw []byte
}

func NewPowBlockInfoFromPosKBlock(posBlock block.Block) *PowBlockInfo {
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

		Raw: buf.Bytes(),

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

func DecodeSignatureScript(bytes []byte) (uint32, string) {
	items := make([][]byte, 0)

	for len(bytes) > 0 {
		l := int(bytes[0])
		end := 1 + l
		items = append(items, bytes[1:end])
		bytes = bytes[end:]
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
