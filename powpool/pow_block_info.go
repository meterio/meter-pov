package powpool

import (
	"bufio"
	"bytes"
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

func NewPowBlockInfoFromBlock(powBlock *PowBlock) *PowBlockInfo {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	powBlock.Serialize(writer)
	return NewPowBlockInfo(buf.Bytes())
}

func NewPowBlockInfo(raw []byte) *PowBlockInfo {
	blk := wire.MsgBlock{}
	blk.Deserialize(strings.NewReader(string(raw)))
	hdr := blk.Header
	info := &PowBlockInfo{
		Version:        uint32(hdr.Version),
		HashPrevBlock:  thor.BytesToBytes32(hdr.PrevBlock.CloneBytes()),
		HashMerkleRoot: thor.BytesToBytes32(hdr.MerkleRoot.CloneBytes()),
		Timestamp:      uint32(hdr.Timestamp.UnixNano()),
		NBits:          hdr.Bits,
		Nonce:          hdr.Nonce,

		// HeaderHash:
		Raw: raw,
		//FIXME: get beneficiary and height from tx information
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
