// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	bls "github.com/dfinlab/meter/crypto/multi_sig"
)

type BlsCommon struct {
	PrivKey bls.PrivateKey //my private key
	PubKey  bls.PublicKey  //my public key

	//global params of BLS
	system  bls.System
	params  bls.Params
	pairing bls.Pairing
}

func NewBlsCommonFromParams(pubKey bls.PublicKey, privKey bls.PrivateKey, system bls.System, params bls.Params, pairing bls.Pairing) *BlsCommon {
	return &BlsCommon{
		PrivKey: privKey,
		PubKey:  pubKey,
		system:  system,
		params:  params,
		pairing: pairing,
	}
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
		PrivKey: PrivKey,
		PubKey:  PubKey,
		system:  system,
		params:  params,
		pairing: pairing,
	}
}

func (cc *BlsCommon) GetSystem() *bls.System {
	return &cc.system
}

func (cc *BlsCommon) GetPrivKey() bls.PrivateKey {
	return cc.PrivKey
}

func (cc *BlsCommon) GetPubKey() *bls.PublicKey {
	return &cc.PubKey
}
