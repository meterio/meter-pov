// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/staking
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

func TestRewardMapToList(t *testing.T) {
	addresses := []meter.Address{meter.MustParseAddress("0xf3dd5c55b96889369f714143f213403464a268a6"),
		meter.MustParseAddress("0xd1186074257f1a6f231c415cdaf7e1f4ae48d51f"),
		meter.MustParseAddress("0xd9b25ec06d7e033b59e7431432ebd0137f5bc886"),
		meter.MustParseAddress("0xd1186074257f1a6f231c415cdaf7e1f4ae48d51f"),
		meter.MustParseAddress("0x5e4ff9a896807e2548b1500d8ff6defbcd2b5493")}

	rewardMap := staking.RewardMap{}
	sum := big.NewInt(0)

	for i, addr := range addresses {
		rewardMap.Add(big.NewInt(1e08+int64(i)), addr)
	}

	for i, in := range rewardMap {
		fmt.Printf("in map === #%v: Address %v, amount %v\n", i, in.Address, in.Amount.Int64())
	}
	sum, info := rewardMap.ToList()

	fmt.Println("the sum", sum)
	fmt.Println("========================")
	for i, in := range info {
		fmt.Printf("#%v: Address %v, amount %v\n", i, in.Address, in.Amount.Int64())
	}

	if info[0].Address.String() != "0x5e4ff9a896807e2548b1500d8ff6defbcd2b5493" ||
		info[1].Address.String() != "0xd1186074257f1a6f231c415cdaf7e1f4ae48d51f" ||
		info[2].Address.String() != "0xd9b25ec06d7e033b59e7431432ebd0137f5bc886" ||
		info[3].Address.String() != "0xf3dd5c55b96889369f714143f213403464a268a6" {
		t.Fail()
	}
}
