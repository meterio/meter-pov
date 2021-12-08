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
		CommitteeMinSize: 2,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  300,
		DiscoServer:      "enode://30fa4a57e203cfef6eb13b2fec75e17849fb9e0be41f7abfc5992955b8c86e4ef484f27efe0d6250ec95a4a871be4b8151727dc86f33d3acfeb92b394e702cbd@13.214.56.167:55555",
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
