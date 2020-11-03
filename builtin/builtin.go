// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/builtin/gen"
	"github.com/dfinlab/meter/builtin/metertracker"
	"github.com/dfinlab/meter/builtin/params"
	"github.com/dfinlab/meter/builtin/prototype"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/dfinlab/meter/xenv"
	"github.com/pkg/errors"
)

// Builtin contracts binding.
var (
	Params       = &paramsContract{mustLoadContract("Params")}
	Meter        = &erc20Contract{mustLoadContract("Meter")}              // erc20 contract 0x0000000000000000000000000000004d65746572
	MeterGov     = &erc20Contract{mustLoadContract("MeterGov")}           // erc20 contract 0x0000000000000000000000004d65746572476f76
	MeterTracker = &meterTrackerContract{mustLoadContract("MeterNative")} // native call contract 0x0000000000000000004d657465724e6174697665
	Executor     = &executorContract{mustLoadContractAddress("Executor",
		meter.InitialExecutorAccount)} //set the excutor address
	Prototype = &prototypeContract{mustLoadContract("Prototype")}
	Extension = &extensionContract{mustLoadContract("Extension")}
	Measure   = mustLoadContract("Measure")
)

type (
	paramsContract       struct{ *contract }
	erc20Contract        struct{ *contract }
	meterTrackerContract struct{ *contract }
	executorContract     struct{ *contract }
	prototypeContract    struct{ *contract }
	extensionContract    struct{ *contract }
)

func (p *paramsContract) Native(state *state.State) *params.Params {
	return params.New(p.Address, state)
}

func (e *meterTrackerContract) Native(state *state.State) *metertracker.MeterTracker {
	return metertracker.New(e.Address, state)
}

func (p *prototypeContract) Native(state *state.State) *prototype.Prototype {
	return prototype.New(p.Address, state)
}

func (p *prototypeContract) Events() *abi.ABI {
	asset := "compiled/PrototypeEvent.abi"
	data := gen.MustAsset(asset)
	abi, err := abi.New(data)
	if err != nil {
		panic(errors.Wrap(err, "load ABI for "+asset))
	}
	return abi
}

type nativeMethod struct {
	abi *abi.Method
	run func(env *xenv.Environment) []interface{}
}

type methodKey struct {
	meter.Address
	abi.MethodID
}

var nativeMethods = make(map[methodKey]*nativeMethod)

// FindNativeCall find native calls.
func FindNativeCall(to meter.Address, input []byte) (*abi.Method, func(*xenv.Environment) []interface{}, bool) {
	methodID, err := abi.ExtractMethodID(input)
	if err != nil {
		return nil, nil, false
	}

	method := nativeMethods[methodKey{to, methodID}]
	if method == nil {
		return nil, nil, false
	}
	return method.abi, method.run, true
}
