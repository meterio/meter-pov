package builtin

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/xenv"
)

const (
	blake2b256WordGas uint64 = 3
	blake2b256Gas     uint64 = 15
)

func init() {
	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_blake2b256", func(env *xenv.Environment) []interface{} {
			var data []byte
			env.ParseArgs(&data)
			env.UseGas(uint64(len(data)+31)/32*blake2b256WordGas + blake2b256Gas)
			output := meter.Blake2b(data)
			return []interface{}{output}
		}},
		{"native_blockID", func(env *xenv.Environment) []interface{} {
			var blockNum uint32
			env.ParseArgs(&blockNum)
			if blockNum >= env.BlockContext().Number {
				return []interface{}{meter.Bytes32{}}
			}

			env.UseGas(meter.SloadGas)
			output := env.Seeker().GetID(blockNum)
			return []interface{}{output}
		}},
		{"native_blockTotalScore", func(env *xenv.Environment) []interface{} {
			var blockNum uint32
			env.ParseArgs(&blockNum)

			if blockNum > env.BlockContext().Number {
				return []interface{}{uint64(0)}
			}

			if blockNum == env.BlockContext().Number {
				return []interface{}{env.BlockContext().TotalScore}
			}

			env.UseGas(meter.SloadGas)
			id := env.Seeker().GetID(blockNum)

			env.UseGas(meter.SloadGas)
			header := env.Seeker().GetHeader(id)
			return []interface{}{header.TotalScore()}
		}},
		{"native_blockTime", func(env *xenv.Environment) []interface{} {
			var blockNum uint32
			env.ParseArgs(&blockNum)

			if blockNum > env.BlockContext().Number {
				return []interface{}{uint64(0)}
			}

			if blockNum == env.BlockContext().Number {
				return []interface{}{env.BlockContext().Time}
			}

			env.UseGas(meter.SloadGas)
			id := env.Seeker().GetID(blockNum)

			env.UseGas(meter.SloadGas)
			header := env.Seeker().GetHeader(id)
			return []interface{}{header.Timestamp()}
		}},
		{"native_blockSigner", func(env *xenv.Environment) []interface{} {
			var blockNum uint32
			env.ParseArgs(&blockNum)

			if blockNum > env.BlockContext().Number {
				return []interface{}{meter.Address{}}
			}

			if blockNum == env.BlockContext().Number {
				return []interface{}{env.BlockContext().Signer}
			}

			env.UseGas(meter.SloadGas)
			id := env.Seeker().GetID(blockNum)

			env.UseGas(meter.SloadGas)
			header := env.Seeker().GetHeader(id)
			signer, _ := header.Signer()
			return []interface{}{signer}
		}},
		{"native_totalSupply", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			output := MeterTracker.Native(env.State()).GetMeterTotalSupply()
			return []interface{}{output}
		}},
		{"native_txProvedWork", func(env *xenv.Environment) []interface{} {
			output := env.TransactionContext().ProvedWork
			return []interface{}{output}
		}},
		{"native_txID", func(env *xenv.Environment) []interface{} {
			output := env.TransactionContext().ID
			return []interface{}{output}
		}},

		{"native_txBlockRef", func(env *xenv.Environment) []interface{} {
			output := env.TransactionContext().BlockRef
			return []interface{}{output}
		}},
		{"native_txExpiration", func(env *xenv.Environment) []interface{} {
			output := env.TransactionContext().Expiration
			return []interface{}{output}
		}},
	}

	abi := Extension.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Extension.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
