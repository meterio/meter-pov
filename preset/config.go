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
	MainnetPresetConfig = &PresetConfig{
		CommitteeMinSize: 3,
		CommitteeMaxSize: 500,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://6624574ce5075b2e1309c2b1f98e0b87f950d759f0fe252a033faf29face42f88a4d159c13e4c3df09eb6b8be9c917361660efe4a59fbd1035761c1f0fe33637@18.138.160.172:55555",
		DiscoTopic:       "metermain",
	}

	TestnetPresetConfig = &PresetConfig{
		CommitteeMinSize: 3,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://f619c6ec38da91609a94982169bf59de025522fb116718770c7e18b38e7fe200f1e3ba937e98a719c297492cfec8857b3274364caa4bb1e388160670ec31bc98@54.254.146.28:55555",
		DiscoTopic:       "metertest",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMinSize: %v, CommitteeMaxSize: %v, DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
