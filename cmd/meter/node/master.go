// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package node

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/meter"
)

type Master struct {
	PrivateKey  *ecdsa.PrivateKey
	PublicKey   *ecdsa.PublicKey
	Beneficiary *meter.Address

	publicBytes []byte
}

func (m *Master) Address() meter.Address {
	return meter.Address(crypto.PubkeyToAddress(m.PrivateKey.PublicKey))
}

func (m *Master) SetPublicBytes(publicBytes []byte) {
	m.publicBytes = publicBytes
}

func (m *Master) GetPublicBytes() []byte {
	return m.publicBytes
}
