package tests

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meterio/meter-pov/meter"
)

func TestCreateContract(t *testing.T) {
	addr := meter.MustParseAddress("0x2afe1451665958ed5b159558d320802fa96f2968")
	nonce := uint32(1885912433)
	contractAddr := meter.EthCreateContractAddress(common.Address(addr), nonce)
	fmt.Println("Created contract: ", contractAddr)
}
