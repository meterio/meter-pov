// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"crypto/ecdsa"
	sha256 "crypto/sha256"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
)

type BlsCommon struct {
	PrivKey bls.PrivateKey //my private key
	PubKey  bls.PublicKey  //my public key

	//global params of BLS
	System      bls.System
	Params      bls.Params
	Pairing     bls.Pairing
	Initialized bool
}

func NewBlsCommon() *BlsCommon {
	params := bls.GenParamsTypeA(160, 512)
	pairing := bls.GenPairing(params)
	system, err := bls.GenSystem(pairing)
	if err != nil {
		return nil
	}

	PubKey, PrivKey, err := bls.GenKeys(system)
	if err != nil {
		return nil
	}
	return &BlsCommon{
		PrivKey:     PrivKey,
		PubKey:      PubKey,
		System:      system,
		Params:      params,
		Pairing:     pairing,
		Initialized: true,
	}
}

func NewBlsCommonFromParams(pubKey bls.PublicKey, privKey bls.PrivateKey, system bls.System, params bls.Params, pairing bls.Pairing) *BlsCommon {
	return &BlsCommon{
		PrivKey:     privKey,
		PubKey:      pubKey,
		System:      system,
		Params:      params,
		Pairing:     pairing,
		Initialized: true,
	}
}

// BLS is implemented by C, memeory need to be freed.
// Signatures also need to be freed but Not here!!!
func (cc *BlsCommon) Destroy() bool {
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

func (cc *BlsCommon) GetSystem() *bls.System {
	return &cc.System
}

func (cc *BlsCommon) GetParams() *bls.Params {
	return &cc.Params
}

func (cc *BlsCommon) GetPairing() *bls.Pairing {
	return &cc.Pairing
}

func (cc *BlsCommon) GetPublicKey() *bls.PublicKey {
	return &cc.PubKey
}

// func (cc *BlsCommon) GetPrivateKey() *bls.PrivateKey {
// 	return &cc.PrivKey
// }

// sign the part of msg
func (cc *BlsCommon) SignMessage(msg []byte) (bls.Signature, [32]byte) {
	hash := sha256.Sum256(msg)
	sig := bls.Sign(hash, cc.PrivKey)
	return sig, hash
}

func (cc *BlsCommon) SignHash(hash [32]byte) []byte {
	sig := bls.Sign(hash, cc.PrivKey)
	return cc.System.SigToBytes(sig)
}

func (cc *BlsCommon) VerifySignature(signature, msgHash, blsPK []byte) bool {
	var fixedMsgHash [32]byte
	copy(fixedMsgHash[:], msgHash[32:])
	pubkey, err := cc.System.PubKeyFromBytes(blsPK)
	defer pubkey.Free()
	if err != nil {
		fmt.Println("pubkey unmarshal failed")
		return false
	}

	sig, err := cc.System.SigFromBytes(signature)
	defer sig.Free()
	if err != nil {
		fmt.Println("signature unmarshal failed")
		return false
	}
	return bls.Verify(sig, fixedMsgHash, pubkey)
}

func (cc *BlsCommon) AggregateSign(sigs []bls.Signature) bls.Signature {
	sig, err := bls.Aggregate(sigs, cc.System)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return sig
}

// all voter sign the same msg.
func (cc *BlsCommon) AggregateVerify(sig bls.Signature, hash [32]byte, pubKeys []bls.PublicKey) (bool, error) {
	hashes := make([][32]byte, 0)
	for i := 0; i < len(pubKeys); i++ {
		hashes = append(hashes, hash)
	}
	return bls.AggregateVerify(sig, hashes, pubKeys)
}

// all voter sign the same msg, so we could aggregate pubkeys together
// and use verify them all
func (cc *BlsCommon) ThresholdVerify(sig bls.Signature, hash [32]byte, pubKeys []bls.PublicKey) (bool, error) {
	if len(pubKeys) <= 0 {
		return false, errors.New("pubkeys are empty")
	}
	aggregatedPubkeys, err := bls.AggregatePubkeys(pubKeys, cc.System)
	defer aggregatedPubkeys.Free()
	if err != nil {
		log.Error("threshold verify failed", "err", err)
		return false, err
	}
	valid := bls.Verify(sig, hash, aggregatedPubkeys)
	return valid, nil
}

func (cc *BlsCommon) SplitPubKey(comboPubKey string) (*ecdsa.PublicKey, *bls.PublicKey) {
	// first part is ecdsa public, 2nd part is bls public key
	split := strings.Split(comboPubKey, ":::")
	// fmt.Println("ecdsa PubKey", split[0], "Bls PubKey", split[1])
	pubKeyBytes, err := b64.StdEncoding.DecodeString(split[0])
	if err != nil {
		panic(fmt.Sprintf("read public key of delegate failed, %v", err))
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		panic(fmt.Sprintf("read public key of delegate failed, %v", err))
	}

	blsPubBytes, err := b64.StdEncoding.DecodeString(split[1])
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}
	blsPub, err := cc.GetSystem().PubKeyFromBytes(blsPubBytes)
	if err != nil {
		panic(fmt.Sprintf("read Bls public key of delegate failed, %v", err))
	}

	return pubKey, &blsPub
}
