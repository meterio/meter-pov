package consensus

import (
	sha256 "crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/meterio/meter-pov/meter"
)

// Vote Message Hash
// "Proposal Block Message: BlockType <8 bytes> Height <16 (8x2) bytes> Round <8 (4x2) bytes>
func BuildBlockVotingHash(blockType uint32, height uint64, id, txsRoot, stateRoot meter.Bytes32) [32]byte {
	c := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(c, blockType)

	h := make([]byte, binary.MaxVarintLen64)
	binary.BigEndian.PutUint64(h, height)

	msg := fmt.Sprintf("%s %s %s %s %s %s %s %s %s %s",
		"BlockType", hex.EncodeToString(c),
		"Height", hex.EncodeToString(h),
		"BlockID", id.String(),
		"TxRoot", txsRoot.String(),
		"StateRoot", stateRoot.String())
	return sha256.Sum256([]byte(msg))
}

// Timeout Vote Message Hash
func BuildTimeoutVotingHash(epoch uint64, round uint32) [32]byte {
	msg := fmt.Sprintf("Timeout: Epoch:%v Round:%v", epoch, round)
	voteHash := sha256.Sum256([]byte(msg))
	return voteHash
}
