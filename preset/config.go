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
		DiscoServer:      "enode://0327d482d2c57c0152f19429b820ea57fa630d4bbe387d4ec289ec23451ecca736c652e82c1d8cec683afa8455fad4afb03a39bde472c8758573e233b54d9c84@disco.meter.io:55555",
		DiscoTopic:       "metermain",
	}

	TestnetPresetConfig = &PresetConfig{
		CommitteeMinSize: 3,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://20cd29b988266dfee6a2afc25086aaf5b2019f44d0845bac9beebe13ee3752bf2380f932fe9c153497d36d866fe591fa246c0baf817547ecc3ebe6014f895792@34.31.117.124:55555",
		DiscoTopic:       "metertest",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMinSize: %v, CommitteeMaxSize: %v, DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
