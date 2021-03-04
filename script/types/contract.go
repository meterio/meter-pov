// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package types

import (
	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/types/gen"
	"github.com/pkg/errors"
)

// Builtin contracts binding.
var (
	ScriptEngine = &scriptEngineContract{mustLoadContract("ScriptEngineEvent")}
)

type (
	scriptEngineContract struct{ *contract }
)

func (p *scriptEngineContract) Events() *abi.ABI {
	asset := "compiled/ScriptEngineEvent.abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load ABI for "+asset))
	}
	return abi
}

type contract struct {
	name    string
	Address meter.Address
	ABI     *abi.ABI
}

func mustLoadContract(name string) *contract {
	asset := "compiled/" + name + ".abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)

	if err != nil {
		panic(errors.Wrap(err, "load ABI for '"+name+"'"))
	}

	return &contract{
		name,
		meter.BytesToAddress([]byte(name)),
		abi,
	}
}
