// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"encoding/hex"

	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/builtin/gen"
	"github.com/dfinlab/meter/meter"
	"github.com/pkg/errors"
)

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

func mustLoadContractAddress(name string, addr meter.Address) *contract {
	asset := "compiled/" + name + ".abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)

	if err != nil {
		panic(errors.Wrap(err, "load ABI for '"+name+"'"))
	}

	return &contract{
		name,
		addr,
		abi,
	}
}

// RuntimeBytecodes load runtime byte codes.
func (c *contract) RuntimeBytecodes() []byte {
	asset := "compiled/" + c.name + ".bin-runtime"
	data, err := hex.DecodeString(string(gen.MustAsset(asset)))
	if err != nil {
		panic(errors.Wrap(err, "load runtime byte code for '"+c.name+"'"))
	}
	return data
}

func (c *contract) NativeABI() *abi.ABI {
	asset := "compiled/" + c.name + "Native.abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load native ABI for '"+c.name+"'"))
	}
	return abi
}

func GetContractABI(name string) *abi.ABI {
	asset := "compiled/" + name + ".abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load ABI for '"+name+"'"))
	}
	return abi
}

func GetContractABIForNewMeterNative() *abi.ABI {
	data := []byte(gen.NewMeterNative_abi)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load ABI for NewMeterNative"))
	}
	return abi
}
