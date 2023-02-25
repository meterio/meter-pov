// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/xenv"
)

var (
	boundEvent, found            = MeterTracker.ABI.EventByName("Bound")
	unboundEvent, _              = MeterTracker.ABI.EventByName("Unbound")
	nativeBucketWithdrawEvent, _ = MeterTracker.ABI.EventByName("NativeBucketWithdraw")
)

func init() {
	log := log15.New("pkg", "metertracker")
	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_mtr_totalSupply", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			supply := MeterTracker.Native(env.State()).GetMeterTotalSupply()
			return []interface{}{supply}
		}},
		{"native_mtr_totalBurned", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			burned := MeterTracker.Native(env.State()).GetMeterTotalBurned()
			return []interface{}{burned}
		}},
		{"native_mtr_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeter(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtr_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeter(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtr_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := MeterTracker.Native(env.State()).SubMeter(meter.Address(args.Addr), args.Amount)
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_mtr_locked_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeterLocked(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtr_locked_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeterLocked(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtr_locked_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := MeterTracker.Native(env.State()).SubMeterLocked(meter.Address(args.Addr), args.Amount)
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_mtrg_totalSupply", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			supply := MeterTracker.Native(env.State()).GetMeterGovTotalSupply()
			return []interface{}{supply}
		}},
		{"native_mtrg_totalBurned", func(env *xenv.Environment) []interface{} {
			env.UseGas(meter.SloadGas)
			burned := MeterTracker.Native(env.State()).GetMeterGovTotalBurned()
			return []interface{}{burned}
		}},
		{"native_mtrg_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeterGov(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtrg_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}
			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeterGov(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtrg_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := false
			if meter.IsTeslaFork1(env.BlockContext().Number) {
				ok = MeterTracker.Native(env.State()).SubMeterGov(meter.Address(args.Addr), args.Amount)
			} else {
				ok = MeterTracker.Native(env.State()).Tesla1_0_SubMeterGov(meter.Address(args.Addr), args.Amount)
			}
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_mtrg_locked_get", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			bal := MeterTracker.Native(env.State()).GetMeterGovLocked(meter.Address(addr))
			return []interface{}{bal}
		}},
		{"native_mtrg_locked_add", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return nil
			}

			env.UseGas(meter.GetBalanceGas)
			if env.State().Exists(meter.Address(args.Addr)) {
				env.UseGas(meter.SstoreResetGas)
			} else {
				env.UseGas(meter.SstoreSetGas)
			}
			MeterTracker.Native(env.State()).AddMeterGovLocked(meter.Address(args.Addr), args.Amount)
			return nil
		}},
		{"native_mtrg_locked_sub", func(env *xenv.Environment) []interface{} {
			var args struct {
				Addr   common.Address
				Amount *big.Int
			}
			env.ParseArgs(&args)
			if args.Amount.Sign() == 0 {
				return []interface{}{true}
			}

			env.UseGas(meter.GetBalanceGas)
			ok := MeterTracker.Native(env.State()).SubMeterGovLocked(meter.Address(args.Addr), args.Amount)
			if ok {
				env.UseGas(meter.SstoreResetGas)
			}
			return []interface{}{ok}
		}},
		{"native_master", func(env *xenv.Environment) []interface{} {
			var addr common.Address
			env.ParseArgs(&addr)

			env.UseGas(meter.GetBalanceGas)
			master := env.State().GetMaster(meter.Address(addr))
			return []interface{}{master}
		}},
		{"native_bucket_open", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner         meter.Address
				CandidateAddr meter.Address
				Amount        *big.Int
			}
			env.ParseArgs(&args)
			log.Info("ENTERED BUCKET_OPEN", "candidateAddr", args.CandidateAddr, "amount", args.Amount)
			if args.Amount.Sign() == 0 {
				return []interface{}{meter.Bytes32{}, "amount is 0"}
			}

			env.UseGas(meter.GetBalanceGas)

			txNonce := env.TransactionContext().Nonce
			clauseIndex := env.ClauseIndex()
			bktID, err := MeterTracker.Native(env.State()).BucketOpen(args.Owner, args.CandidateAddr, args.Amount, env.BlockContext().Time, txNonce+uint64(clauseIndex))
			if err != nil {
				return []interface{}{bktID, err.Error()}
			}

			// emit Bound event
			topics := []meter.Bytes32{
				// meter.Bytes32(boundEvent.ID()),
				meter.BytesToBytes32(args.Owner[:]),
			}
			env.Log(boundEvent, meter.StakingModuleAddr, topics, args.Amount, big.NewInt(int64(meter.MTRG)))
			// env.UseGas(meter.SstoreSetGas)
			return []interface{}{bktID, ""}
		}},
		{"native_bucket_close", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner    meter.Address
				BucketID meter.Bytes32
			}
			env.ParseArgs(&args)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketClose(args.Owner, args.BucketID, env.BlockContext().Time)
			if err != nil {
				return []interface{}{err.Error()}
			}
			// env.UseGas(meter.SstoreSetGas)
			return []interface{}{""}
		}},

		{"native_bucket_deposit", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner    meter.Address
				BucketID meter.Bytes32
				Amount   *big.Int
			}
			env.ParseArgs(&args)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketDeposit(args.Owner, args.BucketID, args.Amount)
			if err != nil {
				return []interface{}{err.Error()}
			}

			// emit Bound event
			topics := []meter.Bytes32{
				// meter.Bytes32(boundEvent.ID()),
				meter.BytesToBytes32(args.Owner[:]),
			}
			env.Log(boundEvent, meter.StakingModuleAddr, topics, args.Amount, big.NewInt(int64(meter.MTRG)))

			// env.UseGas(meter.SstoreSetGas)
			return []interface{}{""}
		}},
		{"native_bucket_withdraw", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner     meter.Address
				BucketID  meter.Bytes32
				Amount    *big.Int
				Recipient meter.Address
			}
			env.ParseArgs(&args)
			env.UseGas(meter.GetBalanceGas)
			txNonce := env.TransactionContext().Nonce
			clauseIndex := env.ClauseIndex()
			ts := env.BlockContext().Time
			bktID, err := MeterTracker.Native(env.State()).BucketWithdraw(args.Owner, args.BucketID, args.Amount, args.Recipient, ts, txNonce+uint64(clauseIndex))
			if err != nil {
				// env.UseGas(meter.SstoreResetGas)
				return []interface{}{bktID, err.Error()}
			}

			// emit NativeBucketWithdraw event
			topics := []meter.Bytes32{
				// meter.Bytes32(boundEvent.ID()),
				meter.BytesToBytes32(args.Owner[:]),
			}
			env.Log(nativeBucketWithdrawEvent, meter.StakingModuleAddr, topics, args.Amount, big.NewInt(int64(meter.MTRG)), args.Recipient)
			return []interface{}{bktID, ""}
		}},
		{"native_bucket_update_candidate", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner            meter.Address
				BucketID         meter.Bytes32
				NewCandidateAddr meter.Address
			}
			env.ParseArgs(&args)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketUpdateCandidate(args.Owner, args.BucketID, args.NewCandidateAddr)
			if err != nil {
				return []interface{}{err.Error()}
			}

			return []interface{}{""}
		}},
	}
	//abi := GetContractABI("NewMeterNative")
	abi := MeterNative_V3_ABI
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{MeterTracker.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
