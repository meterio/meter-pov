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
		DiscoServer:      "enode://e9a70ca78d571b64c6dda32982caf3a12c551a2ebe640791bb31e3c44a6aaaf30af9edfba6338568bc39f46c35f5fa22e463fe841b4ceb01075c6551265383f0@13.215.100.10:55555",
		DiscoTopic:       "mainnet",
	}

	ShoalPresetConfig = &PresetConfig{
		CommitteeMinSize: 3,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://f619c6ec38da91609a94982169bf59de025522fb116718770c7e18b38e7fe200f1e3ba937e98a719c297492cfec8857b3274364caa4bb1e388160670ec31bc98@54.254.146.28:55555",
		DiscoTopic:       "shoal",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMaxSize: %v DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
