// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

// Account is the Thor consensus representation of an account.
// RLP encoded objects are stored in main account trie.
type Account struct {
	Balance      *big.Int
	Energy       *big.Int
	BoundBalance *big.Int
	BoundEnergy  *big.Int
	Master       []byte // master address
	CodeHash     []byte // hash of code
	StorageRoot  []byte // merkle root of the storage trie
}

func (a *Account) String() string {
	s := "Account("
	items := make([]string, 0)
	if a.Balance.Cmp(big.NewInt(0)) > 0 {
		items = append(items, fmt.Sprintf("mtrg:%v", a.Balance))
	}
	if a.Energy.Cmp(big.NewInt(0)) > 0 {
		items = append(items, fmt.Sprintf("mtr:%v", a.Energy))
	}
	if a.BoundBalance.Cmp(big.NewInt(0)) > 0 {
		items = append(items, fmt.Sprintf("bounded:%v", a.BoundBalance))
	}
	if !bytes.Equal(a.Master, []byte{}) {
		items = append(items, fmt.Sprintf("master:%v", hex.EncodeToString(a.Master)))
	}
	if !bytes.Equal(a.CodeHash, []byte{}) {
		items = append(items, fmt.Sprintf("master:%v", hex.EncodeToString(a.CodeHash)))
	}
	if !bytes.Equal(a.StorageRoot, []byte{}) {
		items = append(items, fmt.Sprintf("sroot:%v", hex.EncodeToString(a.StorageRoot)))
	}

	s += strings.Join(items, ", ") + ")"
	return s
}

func (a *Account) DiffString(b *Account) string {
	s := "Account ("
	items := make([]string, 0)
	if a.Balance.Cmp(b.Balance) != 0 {
		items = append(items, fmt.Sprintf("mtrg:%v -> %v", a.Balance, b.Balance))
	}
	if a.Energy.Cmp(b.Energy) != 0 {
		items = append(items, fmt.Sprintf("mtr:%v -> %v", a.Energy, b.Energy))
	}
	if a.BoundBalance.Cmp(b.BoundBalance) != 0 {
		items = append(items, fmt.Sprintf("bounded:%v -> %v", a.BoundBalance, b.BoundBalance))
	}
	if !bytes.Equal(a.Master, b.Master) {
		items = append(items, fmt.Sprintf("master:%v -> %v", hex.EncodeToString(a.Master), hex.EncodeToString(b.Master)))
	}
	if !bytes.Equal(a.CodeHash, b.CodeHash) {
		items = append(items, fmt.Sprintf("master:%v -> %v", hex.EncodeToString(a.CodeHash), hex.EncodeToString(b.CodeHash)))
	}
	if !bytes.Equal(a.StorageRoot, b.StorageRoot) {
		items = append(items, fmt.Sprintf("sroot:%v -> %v", hex.EncodeToString(a.StorageRoot), hex.EncodeToString(b.StorageRoot)))
	}

	s += strings.Join(items, ", ") + ")"
	return s
}

// IsEmpty returns if an account is empty.
// An empty account has zero balance and zero length code hash.
func (a *Account) IsEmpty() bool {
	return a.Balance.Sign() == 0 &&
		a.Energy.Sign() == 0 &&
		a.BoundBalance.Sign() == 0 &&
		a.BoundEnergy.Sign() == 0 &&
		len(a.Master) == 0 &&
		len(a.CodeHash) == 0 &&
		len(a.StorageRoot) == 0
}

var bigE18 = big.NewInt(1e18)

// CalcEnergy calculates energy based on current block time.
func (a *Account) CalcEnergy() *big.Int {
	return a.Energy
}

func emptyAccount() *Account {
	return &Account{Balance: &big.Int{}, Energy: &big.Int{}, BoundBalance: &big.Int{}, BoundEnergy: &big.Int{}}
}

// loadAccount load an account object by address in trie.
// It returns empty account is no account found at the address.
func loadAccount(trie trieReader, addr meter.Address) (*Account, error) {
	data, err := trie.TryGet(addr[:])
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return emptyAccount(), nil
	}
	var a Account
	if err := rlp.DecodeBytes(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// saveAccount save account into trie at given address.
// If the given account is empty, the value for given address is deleted.
func saveAccount(trie trieWriter, addr meter.Address, a *Account) error {
	if a.IsEmpty() {
		// delete if account is empty
		return trie.TryDelete(addr[:])
	}

	data, err := rlp.EncodeToBytes(a)
	if err != nil {
		return err
	}
	return trie.TryUpdate(addr[:], data)
}

// loadStorage load storage data for given key.
func loadStorage(trie trieReader, key meter.Bytes32) (rlp.RawValue, error) {
	return trie.TryGet(key[:])
}

// saveStorage save value for given key.
// If the data is zero, the given key will be deleted.
func saveStorage(trie trieWriter, key meter.Bytes32, data rlp.RawValue) error {
	if len(data) == 0 {
		// release storage if data is zero length
		return trie.TryDelete(key[:])
	}
	return trie.TryUpdate(key[:], data)
}
