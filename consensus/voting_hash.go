package consensus

import (
	sha256 "crypto/sha256"
	"fmt"
)

// Timeout Vote Message Hash
func BuildTimeoutVotingHash(epoch uint64, round uint32) [32]byte {
	msg := fmt.Sprintf("Timeout: Epoch:%v Round:%v", epoch, round)
	voteHash := sha256.Sum256([]byte(msg))
	return voteHash
}
