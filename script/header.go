// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"fmt"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

var (
	ScriptPattern = [4]byte{0xde, 0xad, 0xbe, 0xef} //pattern: deadbeef
)

type Script struct {
	Header  ScriptHeader
	Payload []byte
}

type ScriptHeader struct {
	// Pattern [4]byte
	Version uint32
	ModID   uint32

	cache struct {
		hash  atomic.Value
	}
}

// Version returns the version
func (sh *ScriptHeader) GetVersion() uint32 { return sh.Version }
func (sh *ScriptHeader) GetModID() uint32   { return sh.ModID }

func (sh *ScriptHeader) UniteHash()  (hash meter.Bytes32) {
	if cached := sh.cache.hash.Load(); cached != nil {
		return cached.(meter.Bytes32)
	}
	defer func() { sh.cache.hash.Store(hash) }()

	hw := meter.NewBlake2b()
	err := rlp.Encode(hw, []interface{}{
		sh.Version,
		sh.ModID,
	})
	if err != nil {
		return
	}

	hw.Sum(hash[:0])
	return
}

func (sh *ScriptHeader) ToString() string {
	return fmt.Sprintf("ScriptHeader:::  Version: %v, ModID: %v", sh.Version, sh.ModID)
}

func ScriptEncodeBytes(script *Script) []byte {
	scriptBytes, err := rlp.EncodeToBytes(script)
	if err != nil {
		fmt.Printf("rlp encode failed, %s\n", err.Error())
		return []byte{}
	}

	return scriptBytes
}

func ScriptDecodeFromBytes(bytes []byte) (*Script, error) {
	script := Script{}
	err := rlp.DecodeBytes(bytes, &script)
	return &script, err
}
