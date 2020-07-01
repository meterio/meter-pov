package preset

import (
	"fmt"
)

// The initial version of main network is Edison.
type PresetConfig struct {
	CommitteeMinSize int
	CommitteeMaxSize int
	DelegateMaxSize  int
	DiscoServer      string
	DiscoTopic       string
}

var (
	MainPresetConfig = &PresetConfig{
		CommitteeMinSize: 11,
		CommitteeMaxSize: 50,
		DelegateMaxSize:  100,
		DiscoServer:      "enode://5db49f79c9478cce564401b31bcf23a8cd4fda02b35e0e0e6ca0ba8d79c40aac55ea05335ebf9e5101ed7bc6a0954fdb8bc3c2c77e6ca65513cde319e81aef76@54.255.114.33:55555",
		DiscoTopic:       "mainnet",
	}

	ShoalPresetConfig = &PresetConfig{
		CommitteeMinSize: 21,
		CommitteeMaxSize: 50,
		DelegateMaxSize:  100,
		DiscoServer:      "enode://3011a0740181881c7d4033a83a60f69b68f9aedb0faa784133da84394120ffe9a1686b2af212ffad16fbba88d0ff302f8edb05c99380bd904cbbb96ee4ca8cfb@54.184.14.94:55555",
		DiscoTopic:       "shoal",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMaxSize: %v DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
