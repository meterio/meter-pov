// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block_test

import (
	"math"
	"testing"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/stretchr/testify/assert"
)

func TestGasLimit_IsValid(t *testing.T) {

	tests := []struct {
		gl       uint64
		parentGL uint64
		want     bool
	}{
		{meter.MinGasLimit, meter.MinGasLimit, true},
		{meter.MinGasLimit - 1, meter.MinGasLimit, false},
		{meter.MinGasLimit, meter.MinGasLimit * 2, false},
		{meter.MinGasLimit * 2, meter.MinGasLimit, false},
		{meter.MinGasLimit + meter.MinGasLimit/meter.GasLimitBoundDivisor, meter.MinGasLimit, true},
		{meter.MinGasLimit*2 + meter.MinGasLimit/meter.GasLimitBoundDivisor, meter.MinGasLimit * 2, true},
		{meter.MinGasLimit*2 - meter.MinGasLimit/meter.GasLimitBoundDivisor, meter.MinGasLimit * 2, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).IsValid(tt.parentGL))
	}
}

func TestGasLimit_Adjust(t *testing.T) {

	tests := []struct {
		gl    uint64
		delta int64
		want  uint64
	}{
		{meter.MinGasLimit, 1, meter.MinGasLimit + 1},
		{meter.MinGasLimit, -1, meter.MinGasLimit},
		{math.MaxUint64, 1, math.MaxUint64},
		{meter.MinGasLimit, int64(meter.MinGasLimit), meter.MinGasLimit + meter.MinGasLimit/meter.GasLimitBoundDivisor},
		{meter.MinGasLimit * 2, -int64(meter.MinGasLimit), meter.MinGasLimit*2 - (meter.MinGasLimit*2)/meter.GasLimitBoundDivisor},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).Adjust(tt.delta))
	}
}

func TestGasLimit_Qualify(t *testing.T) {
	tests := []struct {
		gl       uint64
		parentGL uint64
		want     uint64
	}{
		{meter.MinGasLimit, meter.MinGasLimit, meter.MinGasLimit},
		{meter.MinGasLimit - 1, meter.MinGasLimit, meter.MinGasLimit},
		{meter.MinGasLimit, meter.MinGasLimit * 2, meter.MinGasLimit*2 - (meter.MinGasLimit*2)/meter.GasLimitBoundDivisor},
		{meter.MinGasLimit * 2, meter.MinGasLimit, meter.MinGasLimit + meter.MinGasLimit/meter.GasLimitBoundDivisor},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, block.GasLimit(tt.gl).Qualify(tt.parentGL))
	}
}
