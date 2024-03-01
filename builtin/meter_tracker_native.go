// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"log/slog"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/xenv"
)

var (
	boundEvent, _                    = MeterNative_V3_ABI.EventByName("Bound")
	nativeBucketDepositEvent, _      = MeterNative_V4_ABI.EventByName("NativeBucketDeposit")
	nativeBucketWithdrawEvent, _     = MeterNative_V4_ABI.EventByName("NativeBucketWithdraw")
	nativeBucketMergeEvent, _        = MeterNative_V4_ABI.EventByName("NativeBucketMerge")
	nativeBucketOpenEvent, _         = MeterNative_V4_ABI.EventByName("NativeBucketOpen")
	nativeBucketCloseEvent, _        = MeterNative_V4_ABI.EventByName("NativeBucketClose")
	nativeBucketTransferFundEvent, _ = MeterNative_V4_ABI.EventByName("NativeBucketTransferFund")
)

func init() {
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
			slog.Info("native_bucket_open", "owner", args.Owner, "candidateAddr", args.CandidateAddr, "amount", args.Amount)
			if args.Amount.Sign() == 0 {
				return []interface{}{meter.Bytes32{}, "amount is 0"}
			}

			env.UseGas(meter.GetBalanceGas)
			nonce := env.TransactionContext().Nonce + uint64(env.ClauseIndex()) + env.TransactionContext().Counter
			ts := env.BlockContext().Time
			bktID, err := MeterTracker.Native(env.State()).BucketOpen(args.Owner, args.CandidateAddr, args.Amount, ts, nonce)
			if err != nil {
				slog.Error("native_bucket_open failed", "err", err)
				return []interface{}{bktID, err.Error()}
			}
			slog.Info("native_bucket_open success", "bktID", bktID)

			env.TransactionContext().Inc()

			topics := []meter.Bytes32{meter.BytesToBytes32(args.Owner[:])}
			// emit Bound event
			env.Log(boundEvent, meter.StakingModuleAddr, topics, args.Amount, big.NewInt(int64(meter.MTRG)))

			// emit NativeBucketOpen event
			env.Log(nativeBucketOpenEvent, meter.StakingModuleAddr, topics, bktID, args.Amount, big.NewInt(int64(meter.MTRG)))

			// env.UseGas(meter.SstoreSetGas)
			return []interface{}{bktID, ""}
		}},
		{"native_bucket_close", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner    meter.Address
				BucketID meter.Bytes32
			}
			env.ParseArgs(&args)
			slog.Info("native_bucket_close", "owner", args.Owner, "bucketID", args.BucketID)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketClose(args.Owner, args.BucketID, env.BlockContext().Time)
			if err != nil {
				slog.Error("native_bucket_close failed", "err", err)
				return []interface{}{err.Error()}
			}
			slog.Info("native_bucket_close success")

			topics := []meter.Bytes32{meter.BytesToBytes32(args.Owner[:])}
			// emit NativeBucketClose
			env.Log(nativeBucketCloseEvent, meter.StakingModuleAddr, topics, args.BucketID)
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
			slog.Info("native_bucket_deposit", "owner", args.Owner, "bucketID", args.BucketID, "amount", args.Amount)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketDeposit(args.Owner, args.BucketID, args.Amount)
			if err != nil {
				slog.Error("native_bucket_deposit failed", "err", err)
				return []interface{}{err.Error()}
			}
			slog.Info("native_bucket_deposit success")

			topics := []meter.Bytes32{meter.BytesToBytes32(args.Owner[:])}
			// emit Bound event
			env.Log(boundEvent, meter.StakingModuleAddr, topics, args.Amount, big.NewInt(int64(meter.MTRG)))

			// emit NativeBucketDeposit
			env.Log(nativeBucketDepositEvent, meter.StakingModuleAddr, topics, args.BucketID, args.Amount, big.NewInt(int64(meter.MTRG)))

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
			slog.Info("native_bucket_withdraw", "owner", args.Owner, "bucketID", args.BucketID, "amount", args.Amount, "recipient", args.Recipient)
			env.UseGas(meter.GetBalanceGas)
			nonce := env.TransactionContext().Nonce + uint64(env.ClauseIndex()) + env.TransactionContext().Counter
			ts := env.BlockContext().Time
			bktID, err := MeterTracker.Native(env.State()).BucketWithdraw(args.Owner, args.BucketID, args.Amount, args.Recipient, ts, nonce)
			if err != nil {
				slog.Error("native_bucket_withdraw failed", "err", err)
				return []interface{}{bktID, err.Error()}
			}
			slog.Info("native_bucket_withdraw success", "bktID", bktID)

			env.TransactionContext().Inc()

			topics := []meter.Bytes32{meter.BytesToBytes32(args.Owner[:])}
			// emit NativeBucketWithdraw event
			env.Log(nativeBucketWithdrawEvent, meter.StakingModuleAddr, topics, args.BucketID, args.Amount, big.NewInt(int64(meter.MTRG)), args.Recipient, bktID)

			return []interface{}{bktID, ""}
		}},
		{"native_bucket_update_candidate", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner            meter.Address
				BucketID         meter.Bytes32
				NewCandidateAddr meter.Address
			}
			env.ParseArgs(&args)
			slog.Info("native_bucket_update_candidate", "owner", args.Owner, "bucketID", args.BucketID, "newCandidateAddr", args.NewCandidateAddr)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketUpdateCandidate(args.Owner, args.BucketID, args.NewCandidateAddr)
			if err != nil {
				slog.Error("native_bucket_update_candidate failed", "err", err)
				return []interface{}{err.Error()}
			}
			slog.Info("native_bucket_update_candidate success")

			return []interface{}{""}
		}},
		{"native_bucket_transfer_fund", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner        meter.Address
				FromBucketID meter.Bytes32
				ToBucketID   meter.Bytes32
				Amount       *big.Int
			}
			env.ParseArgs(&args)
			slog.Info("native_bucket_transfer_fund", "fromBucketID", args.FromBucketID, "toBucketID", args.ToBucketID, "amount", args.Amount)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketTransferFund(args.Owner, args.FromBucketID, args.ToBucketID, args.Amount)
			if err != nil {
				slog.Error("native_bucket_transfer_fund failed", "err", err)
				return []interface{}{err.Error()}
			}
			slog.Info("native_bucket_transfer_fund success")

			topics := []meter.Bytes32{meter.BytesToBytes32(args.Owner[:])}
			// emit NativeBucketTransferFund
			env.Log(nativeBucketTransferFundEvent, meter.StakingModuleAddr, topics, args.FromBucketID, args.Amount, big.NewInt(int64(meter.MTRG)), args.ToBucketID)
			return []interface{}{""}
		}},
		{"native_bucket_merge", func(env *xenv.Environment) []interface{} {
			var args struct {
				Owner        meter.Address
				FromBucketID meter.Bytes32
				ToBucketID   meter.Bytes32
			}
			env.ParseArgs(&args)
			slog.Info("native_bucket_merge", "fromBucketID", args.FromBucketID, "toBucketID", args.ToBucketID)
			env.UseGas(meter.GetBalanceGas)
			err := MeterTracker.Native(env.State()).BucketMerge(args.Owner, args.FromBucketID, args.ToBucketID)
			if err != nil {
				slog.Error("native_bucket_merge failed", "err", err)
				return []interface{}{err.Error()}
			}
			slog.Info(("native_bucket_merge success"))

			topics := []meter.Bytes32{meter.BytesToBytes32(args.Owner[:])}
			// emit NativeBucketMerge
			env.Log(nativeBucketMergeEvent, meter.StakingModuleAddr, topics, args.FromBucketID, args.ToBucketID)

			return []interface{}{""}
		}},
		{"native_bucket_value", func(env *xenv.Environment) []interface{} {
			var args struct {
				BucketID meter.Bytes32
			}
			env.ParseArgs(&args)
			slog.Info("native_bucket_value", "bucketID", args.BucketID)
			env.UseGas(meter.GetBalanceGas)
			val, _ := MeterTracker.Native(env.State()).BucketValue(args.BucketID)
			return []interface{}{val}
		}},
		{"native_bucket_exists", func(env *xenv.Environment) []interface{} {
			var args struct {
				BucketID meter.Bytes32
			}
			env.ParseArgs(&args)
			slog.Info("native_bucket_exists", "bucketID", args.BucketID)
			bucketList := env.State().GetBucketList()
			bkt := bucketList.Get(args.BucketID)
			if bkt != nil && strings.EqualFold(bkt.ID().String(), args.BucketID.String()) {
				slog.Info("native_bucket_exists true")
				return []interface{}{true}
			}
			slog.Info("native_bucket_exists false")
			return []interface{}{false}
		}},
	}
	//abi := GetContractABI("NewMeterNative")
	abi := MeterNative_V4_ABI
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
