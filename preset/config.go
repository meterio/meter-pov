package preset

import (
	"fmt"

	"github.com/dfinlab/meter/meter"
)

// Fork Release Version
const (
	Edison = iota + 1
)

// The initial version of main network is Edison.
type PresetConfig struct {
	ChainGenesisID   meter.Bytes32
	EdisonEpoch      uint32
	CommitteeMinSize int
	CommitteeMaxSize int
	DelegateMaxSize  int
	DiscoServer      string
	DiscoTopic       string
}

var (
	MainPresetConfig = &PresetConfig{
		ChainGenesisID:   meter.MustParseBytes32("0x00000000642d16e1d39891a93281f92fa76fc527008d79b515e16e3dbc1874b9"), // information
		EdisonEpoch:      0,
		CommitteeMinSize: 21,
		CommitteeMaxSize: 50,
		DelegateMaxSize:  100,
		DiscoServer:      "enode://3011a0740181881c7d4033a83a60f69b68f9aedb0faa784133da84394120ffe9a1686b2af212ffad16fbba88d0ff302f8edb05c99380bd904cbbb96ee4ca8cfb@54.184.14.94:55555",
		DiscoTopic:       "mainnet",
	}

	ShoalPresetConfig = &PresetConfig{
		ChainGenesisID:   meter.MustParseBytes32("0x00000000642d16e1d39891a93281f92fa76fc527008d79b515e16e3dbc1874b9"), // information
		EdisonEpoch:      0,
		CommitteeMinSize: 21,
		CommitteeMaxSize: 50,
		DelegateMaxSize:  100,
		DiscoServer:      "enode://3011a0740181881c7d4033a83a60f69b68f9aedb0faa784133da84394120ffe9a1686b2af212ffad16fbba88d0ff302f8edb05c99380bd904cbbb96ee4ca8cfb@54.184.14.94:55555",
		DiscoTopic:       "shoal",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("EdisonEpoch: %v CommitteeMinSize: %v CommitteeMaxSize: %v DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.EdisonEpoch, p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize,
		p.DiscoServer, p.DiscoTopic)
}

func (p *PresetConfig) IsEdison(curEpoch uint32) bool {
	if curEpoch >= p.EdisonEpoch {
		return true
	} else {
		return false
	}
}
