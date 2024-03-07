package fork11

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestListAsCandidate(t *testing.T) {
	tenv := initRuntimeAfterFork11()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	// list as candidate Holder
	listAsCandidateFunc, found := builtin.ScriptEngine_V3_ABI.MethodByName("listAsCandidate")
	assert.True(t, found)
	openAmount := tests.BuildAmount(2000)
	pubkey := []byte("BHq/NmcbeOS/wEqZGGYOgm79/tLkZy004IFu4gjSt7jiD/fLrJsdKSHqe/oQqT5y78tt6H3zr016hvXte/Ntw70=:::bNpwfSjJbskU1czBj0p/2Y4s03jH8mx9nV+ahX55815HNRG03+nEjOLkh4WcNgUe2MQKVTw83mgNU8Ju79MF8gE=")
	name := []byte("Holder")
	description := []byte("Holder Desc")
	ip := []byte("1.2.3.4")
	port := uint16(8669)
	autobid := uint8(80)
	commission := uint32(8e7)
	data, err := listAsCandidateFunc.EncodeInput(name, description, pubkey, ip, port, openAmount, autobid, commission)
	assert.Nil(t, err)

	txNonce := rand.Uint64()
	bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)
	fmt.Println("bucket id", bktID)

	trx := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data, txNonce, tests.HolderKey)
	receipt, err := tenv.Runtime.ExecuteTransaction(trx)
	topic0, _ := hex.DecodeString("ff4a5e5f8c4699b34aa8eafbd33a98452fd62f799930ca6bde64e21dd42237f7")
	// check bucket ID
	for _, o := range receipt.Outputs {
		for _, e := range o.Events {
			if bytes.Equal(e.Topics[0].Bytes(), topic0) {
				assert.Equal(t, e.Data[:32], bktID.Bytes())
				break
			}
		}
	}
	assert.Nil(t, err)
	assert.False(t, receipt.Reverted)

	// check autobid in bucket list
	bucketList := tenv.State.GetBucketList()
	candidateList := tenv.State.GetCandidateList()
	found = false
	for _, b := range bucketList.Buckets {
		if bytes.Equal(b.BucketID.Bytes(), bktID[:]) {
			found = true
			assert.True(t, b.Autobid == autobid)
			break
		}
	}
	assert.True(t, found)

	// check fields in candidate list
	found = false
	for _, c := range candidateList.Candidates {
		if bytes.Equal(c.Addr.Bytes(), tests.HolderAddr[:]) {
			found = true
			assert.Equal(t, string(pubkey), string(c.PubKey))
			assert.Equal(t, string(name), string(c.Name))
			assert.Equal(t, string(description), string(c.Description))
			assert.Equal(t, string(ip), string(c.IPAddr))
			assert.Equal(t, port, c.Port)
			assert.Equal(t, uint16(commission), uint16(c.Commission))

			break
		}
	}
	assert.True(t, found)

}
