package meter_test

import (
	"fmt"
	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/stretchr/testify/assert"
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
