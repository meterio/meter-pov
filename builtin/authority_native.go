// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/xenv"
)

func init() {
	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_executor", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			addr := meter.BytesToAddress(Params.Native(env.State()).Get(meter.KeyExecutorAddress).Bytes())
			return []interface{}{addr}
		}},
		{"native_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				NodeMaster common.Address
				Endorsor   common.Address
				Identity   common.Hash
			}
			env.ParseArgs(&args)

			env.UseGas(meter.SloadGas)
			ok := Authority.Native(env.State()).Add(
				meter.Address(args.NodeMaster),
				meter.Address(args.Endorsor),
				meter.Bytes32(args.Identity))

			if ok {
				env.UseGas(meter.SstoreSetGas)
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_revoke", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(meter.SloadGas)
			ok := Authority.Native(env.State()).Revoke(meter.Address(nodeMaster))
			if ok {
				env.UseGas(meter.SstoreResetGas * 3)
			}
			return []interface{}{ok}
		}},
		{"native_get", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(meter.SloadGas * 2)
			listed, endorsor, identity, active := Authority.Native(env.State()).Get(meter.Address(nodeMaster))

			return []interface{}{listed, endorsor, identity, active}
		}},
		{"native_first", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			if nodeMaster := Authority.Native(env.State()).First(); nodeMaster != nil {
				return []interface{}{*nodeMaster}
			}
			return []interface{}{meter.Address{}}
		}},
		{"native_next", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(meter.SloadGas)
			if next := Authority.Native(env.State()).Next(meter.Address(nodeMaster)); next != nil {
				return []interface{}{*next}
			}
			return []interface{}{meter.Address{}}
		}},
		{"native_isEndorsed", func(env *xenv.Environment) []interface{} {
			var nodeMaster common.Address
			env.ParseArgs(&nodeMaster)

			env.UseGas(meter.SloadGas * 2)
			listed, endorsor, _, _ := Authority.Native(env.State()).Get(meter.Address(nodeMaster))
			if !listed {
				return []interface{}{false}
			}

			env.UseGas(meter.GetBalanceGas)
			bal := env.State().GetBalance(endorsor)

			env.UseGas(meter.SloadGas)
			endorsement := Params.Native(env.State()).Get(meter.KeyProposerEndorsement)
			return []interface{}{bal.Cmp(endorsement) >= 0}
		}},
	}
	abi := Authority.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Authority.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
