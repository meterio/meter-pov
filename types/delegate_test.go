package types

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	crypto "github.com/dfinlab/go-zdollar/crypto"
	"github.com/dfinlab/go-zdollar/crypto/ed25519"

	cmn "github.com/dfinlab/go-zdollar/libs/common"
)

func TestDelegateSetBasic(t *testing.T) {
	for _, vset := range []*DelegateSet{NewDelegateSet([]*Delegate{}), NewDelegateSet(nil)} {

		assert.EqualValues(t, vset, vset.Copy())
		assert.False(t, vset.HasAddress([]byte("some val")))
		idx, val := vset.GetByAddress([]byte("some val"))
		assert.Equal(t, -1, idx)
		assert.Nil(t, val)
		addr, val := vset.GetByIndex(-100)
		assert.Nil(t, addr)
		assert.Nil(t, val)
		addr, val = vset.GetByIndex(0)
		assert.Nil(t, addr)
		assert.Nil(t, val)
		addr, val = vset.GetByIndex(100)
		assert.Nil(t, addr)
		assert.Nil(t, val)
		assert.Zero(t, vset.Size())
		assert.Equal(t, int64(0), vset.TotalVotingPower())
		assert.Nil(t, vset.Hash())

		// add
		val = randDelegate_()
		assert.True(t, vset.Add(val))
		assert.True(t, vset.HasAddress(val.Address))
		idx, val2 := vset.GetByAddress(val.Address)
		assert.Equal(t, 0, idx)
		assert.Equal(t, val, val2)
		addr, val2 = vset.GetByIndex(0)
		assert.Equal(t, []byte(val.Address), addr)
		assert.Equal(t, val, val2)
		assert.Equal(t, 1, vset.Size())
		assert.Equal(t, val.VotingPower, vset.TotalVotingPower())
		assert.NotNil(t, vset.Hash())

		// update
		assert.False(t, vset.Update(randDelegate_()))
		val.VotingPower = 100
		assert.True(t, vset.Update(val))

		// remove
		val2, removed := vset.Remove(randDelegate_().Address)
		assert.Nil(t, val2)
		assert.False(t, removed)
		val2, removed = vset.Remove(val.Address)
		assert.Equal(t, val.Address, val2.Address)
		assert.True(t, removed)
	}
}

func TestDelegateSetCopy(t *testing.T) {
	vset := randDelegateSet(10)
	vsetHash := vset.Hash()
	if len(vsetHash) == 0 {
		t.Fatalf("DelegateSet had unexpected zero hash")
	}

	vsetCopy := vset.Copy()
	vsetCopyHash := vsetCopy.Hash()

	if !bytes.Equal(vsetHash, vsetCopyHash) {
		t.Fatalf("DelegateSet copy had wrong hash. Orig: %X, Copy: %X", vsetHash, vsetCopyHash)
	}
}

func BenchmarkDelegateSetCopy(b *testing.B) {
	b.StopTimer()
	vset := NewDelegateSet([]*Delegate{})
	for i := 0; i < 1000; i++ {
		privKey := crypto.GenPrivKeySecp256k1()
		pubKey := privKey.PubKey()
		val := NewDelegate(pubKey, 0)
		if !vset.Add(val) {
			panic("Failed to add Delegate")
		}
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		vset.Copy()
	}
}

//-------------------------------------------------------------------

func newDelegate(address []byte, power int64) *Delegate {
	return &Delegate{Address: address, VotingPower: power}
}

func randDelegatePubKey() crypto.PubKey {
	var pubKey [32]byte
	copy(pubKey[:], cmn.RandBytes(32))

	priv_key := crypto.GenPrivKeySecp256k1()
	return (priv_key.PubKey())
}

func randDelegate_() *Delegate {
	val := NewDelegate(randDelegatePubKey(), cmn.RandInt64())
	val.Accum = cmn.RandInt64()
	return val
}

func randDelegateSet(numDelegates int) *DelegateSet {
	Delegates := make([]*Delegate, numDelegates)
	for i := 0; i < numDelegates; i++ {
		Delegates[i] = randDelegate_()
	}
	return NewDelegateSet(Delegates)
}

func (valSet *DelegateSet) toBytes() []byte {
	bz, err := cdc.MarshalBinary(valSet)
	if err != nil {
		panic(err)
	}
	return bz
}

func (valSet *DelegateSet) fromBytes(b []byte) {
	err := cdc.UnmarshalBinary(b, &valSet)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		panic(err)
	}
}
