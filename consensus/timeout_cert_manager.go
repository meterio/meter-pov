package consensus

import (
	"fmt"
	"sync"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	cmn "github.com/dfinlab/meter/libs/common"
)

type timeoutID struct {
	Height uint64
	Round  uint64
}

type timeoutVal struct {
	Counter   uint64
	PeerID    []byte
	PeerIndex int
	MsgHash   [32]byte
	Signature bls.Signature
}

type PMTimeoutCertManager struct {
	sync.RWMutex
	pacemaker *Pacemaker

	cache     map[timeoutID][]*timeoutVal
	bitArrays map[timeoutID]*cmn.BitArray
}

func newPMTimeoutCertManager(pacemaker *Pacemaker) *PMTimeoutCertManager {
	return &PMTimeoutCertManager{
		pacemaker: pacemaker,
		cache:     make(map[timeoutID][]*timeoutVal),
		bitArrays: make(map[timeoutID]*cmn.BitArray),
	}
}

func (tm *PMTimeoutCertManager) collectSignature(newViewMsg *PMNewViewMessage) error {
	tm.Lock()
	defer tm.Unlock()

	if newViewMsg.Reason == RoundTimeout {
		index := newViewMsg.PeerIndex
		// append signature only if it doesn't exist
		height := newViewMsg.TimeoutHeight
		round := newViewMsg.TimeoutRound

		var bitArray *cmn.BitArray
		id := timeoutID{Height: height, Round: round}
		bitArray, ok := tm.bitArrays[id]
		if !ok {
			bitArray = cmn.NewBitArray(tm.pacemaker.csReactor.committeeSize)
			tm.bitArrays[id] = bitArray
		}

		if bitArray.GetIndex(index) == false {
			bitArray.SetIndex(index, true)
			sig, err := tm.pacemaker.csReactor.csCommon.system.SigFromBytes(newViewMsg.PeerSignature)
			if err != nil {
				tm.pacemaker.logger.Error("error convert signature", "err", err)
			}
			var vals []*timeoutVal
			vals, ok := tm.cache[id]
			if !ok {
				vals = make([]*timeoutVal, 0)
			}
			fmt.Println("APPEND: ", len(tm.cache[id]))
			tm.cache[id] = append(vals, &timeoutVal{
				// TODO: set counter
				Counter:   newViewMsg.TimeoutCounter,
				PeerID:    newViewMsg.PeerID,
				PeerIndex: newViewMsg.PeerIndex,
				MsgHash:   newViewMsg.SignedMessageHash,
				Signature: sig,
			})
			fmt.Println("AFTER APPEND", len(tm.cache[id]))
		}
	}
	return nil
}

func (tm *PMTimeoutCertManager) count(height, round uint64) int {
	tm.RLock()
	defer tm.RUnlock()
	if bitArray, ok := tm.bitArrays[timeoutID{height, round}]; ok {
		return bitArray.Count()
	}
	return 0
}

func (tm *PMTimeoutCertManager) getTimeoutCert(height, round uint64) *PMTimeoutCert {
	id := timeoutID{height, round}
	vals, ok := tm.cache[id]
	if !ok {
		return nil
	}
	bitArray, ok := tm.bitArrays[id]
	if !ok {
		return nil
	}

	var sigs []bls.Signature
	for _, v := range vals {
		sigs = append(sigs, v.Signature)
	}
	aggSig := tm.pacemaker.csReactor.csCommon.AggregateSign(sigs)
	aggSigBytes := tm.pacemaker.csReactor.csCommon.system.SigToBytes(aggSig)
	return &PMTimeoutCert{
		TimeoutHeight: height,
		TimeoutRound:  round,
		//TODO: better way?
		TimeoutCounter:  uint32(vals[0].Counter),
		TimeoutBitArray: bitArray,
		TimeoutAggSig:   aggSigBytes,
	}
}
