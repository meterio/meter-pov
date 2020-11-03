// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package meter.test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/dfinlab/meter/meter"
)

func TestBloom(t *testing.T) {

	itemCount := 100
	bloom := meter.NewBloom(meter.EstimateBloomK(itemCount))

	for i := 0; i < itemCount; i++ {
		bloom.Add([]byte(fmt.Sprintf("%v", i)))
	}

	for i := 0; i < itemCount; i++ {
		assert.Equal(t, true, bloom.Test([]byte(fmt.Sprintf("%v", i))))
	}
}
