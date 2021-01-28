// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	sha256 "crypto/sha256"
	"fmt"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
)

type ConsensusCommon struct {
	PrivKey bls.PrivateKey //my private key
	PubKey  bls.PublicKey  //my public key

	//global params of BLS
	System      bls.System
	Params      bls.Params
	Pairing     bls.Pairing
	Initialized bool
}

// BLS is implemented by C, memeory need to be freed.
// Signatures also need to be freed but Not here!!!
func (cc *ConsensusCommon) Destroy() bool {
	if !cc.Initialized {
		fmt.Println("BLS is not initialized!")
		return false
	}

	cc.PubKey.Free()
	cc.PrivKey.Free()
	cc.System.Free()
	cc.Pairing.Free()
	cc.Params.Free()

	cc.Initialized = false
	return true
}

func (cc *ConsensusCommon) GetSystem() *bls.System {
	return &cc.System
}

func (cc *ConsensusCommon) GetParams() *bls.Params {
	return &cc.Params
}

func (cc *ConsensusCommon) GetPairing() *bls.Pairing {
	return &cc.Pairing
}

func (cc *ConsensusCommon) GetPublicKey() *bls.PublicKey {
	return &cc.PubKey
}

// func (cc *ConsensusCommon) GetPrivateKey() *bls.PrivateKey {
// 	return &cc.PrivKey
// }

// sign the part of msg
func (cc *ConsensusCommon) Hash256Msg(msg []byte) [32]byte {
	return sha256.Sum256(msg)
}

// sign the part of msg
func (cc *ConsensusCommon) SignMessage(msg []byte) (bls.Signature, [32]byte) {
	hash := sha256.Sum256(msg)
	sig := bls.Sign(hash, cc.PrivKey)
	return sig, hash
}

// the return with slice byte
func (cc *ConsensusCommon) SignMessage2(msg []byte) ([]byte, [32]byte) {
	hash := sha256.Sum256(msg)
	sig := bls.Sign(hash, cc.PrivKey)
	return cc.System.SigToBytes(sig), hash
}

func (cc *ConsensusCommon) VerifySignature(signature, msgHash, blsPK []byte) bool {
	var fixedMsgHash [32]byte
	copy(fixedMsgHash[:], msgHash[32:])
	pubkey, err := cc.System.PubKeyFromBytes(blsPK)
	if err != nil {
		fmt.Println("pubkey unmarshal failed")
		return false
	}

	sig, err := cc.System.SigFromBytes(signature)
	if err != nil {
		fmt.Println("signature unmarshal failed")
		return false
	}
	return bls.Verify(sig, fixedMsgHash, pubkey)
}

func (cc *ConsensusCommon) AggregateSign(sigs []bls.Signature) bls.Signature {
	sig, err := bls.Aggregate(sigs, cc.System)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return sig
}

func (cc *ConsensusCommon) AggregateSign2(sigs []bls.Signature) []byte {
	sig, err := bls.Aggregate(sigs, cc.System)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return cc.System.SigToBytes(sig)
}

// all voter sign the same msg.
func (cc *ConsensusCommon) AggregateVerify(sig bls.Signature, hashes [][32]byte, pubKeys []bls.PublicKey) (bool, error) {
	return bls.AggregateVerify(sig, hashes, pubKeys)
}
