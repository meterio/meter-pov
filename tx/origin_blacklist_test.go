// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx

import (
	"testing"

	"github.com/meterio/meter-pov/meter"
	"github.com/stretchr/testify/assert"
)

func TestIsOriginBlacklisted(t *testing.T) {
	assert.True(t, IsOriginBlacklisted(meter.MustParseAddress("0x0e369a2e02912dba872e72d6c0b661e9617e0d9c")))
	assert.False(t, IsOriginBlacklisted(meter.ZeroAddress))
	assert.False(t, IsOriginBlacklisted(meter.MustParseAddress("0x0000000000000000000000000000000000000001")))
}
