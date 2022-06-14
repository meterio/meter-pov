package staking

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/meterio/meter-pov/meter"
)

func TestBucket(t *testing.T) {
	bkts := make([]*Bucket, 0)
	addr := meter.MustParseAddress("0x08ebea6584b3d9bf6fbcacf1a1507d00a61d95b7")
	b1 := NewBucket(addr, addr, big.NewInt(100), 1, 0, 0, 0, 1, 1)
	b2 := NewBucket(addr, addr, big.NewInt(200), 1, 0, 0, 0, 2, 2)
	b3 := NewBucket(addr, addr, big.NewInt(300), 1, 0, 0, 0, 3, 3)
	b4 := NewBucket(addr, addr, big.NewInt(400), 1, 0, 0, 0, 4, 4)
	b5 := NewBucket(addr, addr, big.NewInt(500), 1, 0, 0, 0, 5, 5)
	b6 := NewBucket(addr, addr, big.NewInt(600), 1, 0, 0, 0, 6, 6)

	bkts = append(bkts, b1, b2, b3, b4, b5, b6)
	list := newBucketList(bkts)

	for i := 0; i < len(list.buckets); i++ {
		fmt.Println("i =", i)
		for j, b := range list.buckets {
			fmt.Print("[", j, "]", b.Value, ",")
		}

		fmt.Println("")
		b := list.buckets[i]
		fmt.Println("visit bucket #", i, b.Value)
		if b.Value.Cmp(big.NewInt(200)) == 0 || b.Value.Cmp(big.NewInt(400)) == 0 {
			fmt.Println("remove bucket:", b.Value)
			list.Remove(b.ID())
			i--
		}
		fmt.Println("--------------------------------")
	}
}
