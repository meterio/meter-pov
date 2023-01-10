package reward_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"math/big"
	"math/rand"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/reward"
	"github.com/meterio/meter-pov/script/auction"
	"github.com/meterio/meter-pov/tx"
	"github.com/stretchr/testify/assert"
)

func buildAuctionCB() *meter.AuctionCB {
	cb := &meter.AuctionCB{
		AuctionID:   meter.BytesToBytes32([]byte("test-auction")),
		StartHeight: 1234,
		StartEpoch:  1,
		EndHeight:   4321,
		EndEpoch:    2,
		Sequence:    1,
		RlsdMTRG:    big.NewInt(0), //released mtrg
		RsvdMTRG:    big.NewInt(0), // reserved mtrg
		RsvdPrice:   big.NewInt(0),
		CreateTime:  1234,

		//changed fields after auction start
		RcvdMTR:    big.NewInt(0),
		AuctionTxs: make([]*meter.AuctionTx, 0),
	}
	for i := 1; i < 0; i++ {
		seq := i
		tx := &meter.AuctionTx{
			TxID:      meter.BytesToBytes32([]byte("test-" + strconv.Itoa(seq))),
			Address:   meter.BytesToAddress([]byte("address-" + strconv.Itoa(seq))),
			Amount:    big.NewInt(1234),
			Type:      auction.AUTO_BID,
			Timestamp: rand.Uint64(),
			Nonce:     rand.Uint64(),
		}
		cb.AuctionTxs = append(cb.AuctionTxs, tx)
	}
	return cb
}

func buildAutobidTx(rewardMap reward.RewardMap) *tx.Transaction {
	autobids := rewardMap.GetAutobidList()
	bestNum := uint32(123)
	tx := reward.BuildAutobidTx(autobids, byte(82), bestNum)
	return tx
}

func TestRunAutobidTx(t *testing.T) {
	rt := initRuntime()
	rewardMap := buildRewardMap()

	start := time.Now()
	tx := buildAutobidTx(rewardMap)
	fmt.Println("Build Autobid tx elapsed: ", meter.PrettyDuration(time.Since(start)))

	start = time.Now()
	_, err := rt.ExecuteTransaction(tx)
	fmt.Println("Run Autobid tx elapsed: ", meter.PrettyDuration(time.Since(start)))
	assert.Nil(t, err)
}
