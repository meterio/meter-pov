// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntInSlice(t *testing.T) {
	assert.True(t, IntInSlice(1, []int{1, 2, 3}))
	assert.False(t, IntInSlice(4, []int{1, 2, 3}))
	assert.True(t, IntInSlice(0, []int{0}))
	assert.False(t, IntInSlice(0, []int{}))
}
