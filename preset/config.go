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
		CommitteeMinSize: 5,
		CommitteeMaxSize: 500,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://d34b1fd5aa18e5885cb9d91dfc6721888c9a35f4dfd7fe1e9387e7ebd3e79647703db6d8ffd44ab5c4cec3f45d5766c3f8cf37df84d7c3e713ae8a1470df6cae@3.0.39.82:55555",
		DiscoTopic:       "metermain",
	}

	TestnetPresetConfig = &PresetConfig{
		CommitteeMinSize: 3,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://83fe5b285c6442ce54276e6177637f72da3288a947840bba2247d6a87af466fca745b727f349a6d8aec12f6db46f44ed7bc42bfb1cbe22bb17c8f3659c3f607a@35.81.5.183:55555",
		DiscoTopic:       "metertest",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMinSize: %v, CommitteeMaxSize: %v, DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
