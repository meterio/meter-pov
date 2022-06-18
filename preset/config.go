// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

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
		CommitteeMaxSize: 500,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://3d7fd902f61ce3a63239a7e1296be976bbd40bb79902669e8501a5c5c50e7c1070d961d87d668b2eb11001a1fdd01e78b1b97b4c050db9e7d10743de464c65f8@175.41.153.121:55555",
		DiscoTopic:       "mainnet",
	}

	ShoalPresetConfig = &PresetConfig{
		CommitteeMinSize: 3,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://5b657dfef1cc6a0f0fe0f15dd6ca160f5cbe2f3ad56faaca7fd933fc552c21bedb3482aff9da6eb73c63731d452d7928db66d16aba2970a3f4386c37a5c740fe@46.137.198.231:55555",
		DiscoTopic:       "shoal",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMaxSize: %v DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
