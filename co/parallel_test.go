// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package co_test

import (
	"testing"
	"time"

	"github.com/dfinlab/meter/co"
)

func TestParallel(t *testing.T) {
	n := 50
	fn := func() {
		time.Sleep(time.Millisecond * 20)
	}

	startTime := time.Now().UnixNano()
	for i := 0; i < n; i++ {
		fn()
	}
	t.Log("non-parallel", time.Duration(time.Now().UnixNano()-startTime))

	startTime = time.Now().UnixNano()
	<-co.Parallel(func(queue chan<- func()) {
		for i := 0; i < n; i++ {
			queue <- fn
		}
	})
	t.Log("parallel", time.Duration(time.Now().UnixNano()-startTime))
}
