package fork11

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/meterio/meter-pov/builtin"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/tests"
	"github.com/stretchr/testify/assert"
)

func TestCandidateUpdate(t *testing.T) {
	tenv := initRuntimeAfterFork11()
	scriptEngineAddr := meter.ScriptEngineSysContractAddr

	bktID := tests.BucketID(tests.Cand2Addr, 0, 0)
	fmt.Println("bucket id", bktID)

	candidateUpdateFunc, found := builtin.ScriptEngine_V3_ABI.MethodByName("candidateUpdate")
	assert.True(t, found)
	pubkey2 := []byte("BBjBRy5w1OP/9fgxbzj7pQLeawAOffn+eRPnB/dn3++Aaj1072+8FWcPg1dmHF2CflD1T5BJJ7AzpqTHrVgRqBA=:::QNMcarhu9s6/lGc3+1L1ibbIQNMa4+wXTx/uttJjUo2YSuTV6hkpgkznSDtxf3SIA0jxbyyyf6RUZm9t5vZZFwE=")
	name2 := []byte("After Name")
	description2 := []byte("After Desc")
	ip2 := []byte("4.3.2.1")
	port2 := uint16(8670)
	autobid2 := uint8(100)
	commission2 := uint32(5e7)
	data2, err := candidateUpdateFunc.EncodeInput(name2, description2, pubkey2, ip2, port2, autobid2, commission2)
	assert.Nil(t, err)

	txNonce2 := rand.Uint64()
	// bktID := tests.BucketID(tests.HolderAddr, tenv.CurrentTS, txNonce+0)

	trx2 := tests.BuildCallTx(tenv.ChainTag, 0, &scriptEngineAddr, data2, txNonce2, tests.Cand2Key)
	receipt2, err := tenv.Runtime.ExecuteTransaction(trx2)
	assert.Nil(t, err)
	assert.False(t, receipt2.Reverted)

	bucketList := tenv.State.GetBucketList()
	candidateList := tenv.State.GetCandidateList()
	found = false
	for _, b := range bucketList.Buckets {
		if bytes.Equal(b.BucketID.Bytes(), bktID[:]) {
			found = true
			// assert.True(t, b.Autobid == autobid2)
			break
		}
	}
	assert.True(t, found)

	// check fields in candidate list
	found = false
	for _, c := range candidateList.Candidates {
		if bytes.Equal(c.Addr.Bytes(), tests.Cand2Addr[:]) {
			found = true
			assert.Equal(t, string(pubkey2), string(c.PubKey))
			assert.Equal(t, string(name2), string(c.Name))
			assert.Equal(t, string(description2), string(c.Description))
			assert.Equal(t, string(ip2), string(c.IPAddr))
			assert.Equal(t, port2, c.Port)
			assert.Equal(t, uint16(commission2), uint16(c.Commission))

			break
		}
	}
	assert.True(t, found)
}
