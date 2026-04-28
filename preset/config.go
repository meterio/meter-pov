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
		DiscoServer:      "enode://dfff1402841ae742721bec4b0a19d8f375e375e25292a7b64e40a79cd42afdfd5868e0b9d9779b3537fe7b328552ff804ce8a0bd79edb46ba949af0a69f8f9a9@35.224.168.31:55555",
		DiscoTopic:       "metermain",
	}

	TestnetPresetConfig = &PresetConfig{
		CommitteeMinSize: 1,
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
