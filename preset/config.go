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
		CommitteeMaxSize: 300,
		DelegateMaxSize:  300,
		DiscoServer:      "enode://7d4835bebc2d6e515ba6bd61c75f4b68870d34a10450b6bcee268d6c23012e4a344c6ae74bd52b6eb74e0a50261fdeeeaf462e13fdcbe03287349a114caca4a8@54.179.74.104:55555",
		DiscoTopic:       "mainnet",
	}

	ShoalPresetConfig = &PresetConfig{
		CommitteeMinSize: 11,
		CommitteeMaxSize: 300,
		DelegateMaxSize:  500,
		DiscoServer:      "enode://edef07cf4108b1b8be09b34290cda09e490a23b1380cbb20cefc250ebaf2470dd470449eb080c18f48c7f1618d7b17f815729dd89ac51193a2f305cf2dee6182@13.228.91.172:55555",
		DiscoTopic:       "shoal",
	}
)

func (p *PresetConfig) ToString() string {
	return fmt.Sprintf("CommitteeMaxSize: %v DelegateMaxSize: %v DiscoServer: %v : DiscoTopic%v",
		p.CommitteeMinSize, p.CommitteeMaxSize, p.DelegateMaxSize, p.DiscoServer, p.DiscoTopic)
}
