// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package tx

import "github.com/meterio/meter-pov/meter"

// blacklistedOrigins is a compile-time transaction admission and proposal
// policy. Add addresses to this map to prevent official nodes from accepting,
// relaying, or proposing transactions signed by those addresses.
//
// This policy must not be used while validating or executing blocks received
// from the network. Doing so would turn the list into a consensus rule and
// require coordinated fork activation.
var blacklistedOrigins = map[meter.Address]struct{}{
	meter.MustParseAddress("0x0e369a2e02912dba872e72d6c0b661e9617e0d9c"): {},
}

// IsOriginBlacklisted reports whether origin is blocked by the local
// transaction admission and proposal policy.
func IsOriginBlacklisted(origin meter.Address) bool {
	_, found := blacklistedOrigins[origin]
	return found
}
