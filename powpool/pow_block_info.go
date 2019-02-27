package powpool

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
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
	RewardCoef  uint64

	// Raw block
	Raw []byte
}

func NewPowBlockInfoFromBlock(powBlock *wire.MsgBlock) *PowBlockInfo {
	buf := bytes.NewBufferString("")
	powBlock.Serialize(buf)
	return NewPowBlockInfo(buf.Bytes())
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
	hdr := blk.Header
	prevBytes := reverse(hdr.PrevBlock.CloneBytes())
	merkleRootBytes := reverse(hdr.MerkleRoot.CloneBytes())

	var height uint32
	beneficiaryAddr := "0x"
	if len(blk.Transactions) == 1 && len(blk.Transactions[0].TxIn) == 1 {
		ss := blk.Transactions[0].TxIn[0].SignatureScript
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

		// HeaderHash:
		Raw: raw,
		//FIXME: get beneficiary and height from tx information
		PowHeight:   height,
		Beneficiary: beneficiary,
	}

	info.HeaderHash = info.HashID()
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
