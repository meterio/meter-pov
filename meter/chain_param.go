package meter

import (
	"fmt"
)

// Fork Release Version
// Edision: The initial Basic Release. Features include
const (
	Edison = iota + 1
)

// Genesis hashes to enforce below configs on.
var (
	MainnetGenesisHash = MustParseBytes32("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
)

var (
	// MainnetChainConfig is the chain parameters to run a node on the main network.
	MainnetChainConfig = &ChainConfig{
		ChainGenesisID: MainnetGenesisHash,
		EdisonEpoch:    0,
	}
)

// ChainConfig is the core config which determines the blockchain settings.
//
type ChainConfig struct {
	ChainGenesisID Bytes32
	EdisonEpoch    uint64
}

func (c *ChainConfig) ToString() string {
	return fmt.Sprintf("ChainGenesisID: %v, EdisonEpoch: %v", c.ChainGenesisID, c.EdisonEpoch)
}

// TBD: There would be more rules when 2nd fork is there.
func (p *ChainConfig) IsEdison(curEpoch uint64) bool {
	if curEpoch >= p.EdisonEpoch {
		return true
	} else {
		return false
	}
}

func IsMainChainEdison(curEpoch uint64) bool {
	return MainnetChainConfig.IsEdison(curEpoch)
}
