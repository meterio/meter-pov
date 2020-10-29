// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package builtin

import (
	"math/big"

	"github.com/dfinlab/meter/abi"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/xenv"
	"github.com/ethereum/go-ethereum/common"
)

func init() {

	events := Prototype.Events()

	mustEventByName := func(name string) *abi.Event {
		if event, found := events.EventByName(name); found {
			return event
		}
		panic("event not found")
	}

	masterEvent := mustEventByName("$Master")
	creditPlanEvent := mustEventByName("$CreditPlan")
	userEvent := mustEventByName("$User")
	sponsorEvent := mustEventByName("$Sponsor")

	defines := []struct {
		name string
		run  func(env *xenv.Environment) []interface{}
	}{
		{"native_master", func(env *xenv.Environment) []interface{} {
			var self common.Address
			env.ParseArgs(&self)

			env.UseGas(meter.GetBalanceGas)
			master := env.State().GetMaster(meter.Address(self))

			return []interface{}{master}
		}},
		{"native_setMaster", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self      common.Address
				NewMaster common.Address
			}
			env.ParseArgs(&args)

			env.UseGas(meter.SstoreResetGas)
			env.State().SetMaster(meter.Address(args.Self), meter.Address(args.NewMaster))

			env.Log(masterEvent, meter.Address(args.Self), nil, args.NewMaster)
			return nil
		}},
		{"native_balanceAtBlock", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self        common.Address
				BlockNumber uint32
			}
			env.ParseArgs(&args)
			ctx := env.BlockContext()

			if args.BlockNumber > ctx.Number {
				return []interface{}{&big.Int{}}
			}

			if ctx.Number-args.BlockNumber > meter.MaxBackTrackingBlockNumber {
				return []interface{}{&big.Int{}}
			}

			if args.BlockNumber == ctx.Number {
				env.UseGas(meter.GetBalanceGas)
				val := env.State().GetBalance(meter.Address(args.Self))
				return []interface{}{val}
			}

			env.UseGas(meter.SloadGas)
			blockID := env.Seeker().GetID(args.BlockNumber)

			env.UseGas(meter.SloadGas)
			header := env.Seeker().GetHeader(blockID)

			env.UseGas(meter.SloadGas)
			state := env.State().Spawn(header.StateRoot())

			env.UseGas(meter.GetBalanceGas)
			val := state.GetBalance(meter.Address(args.Self))

			return []interface{}{val}
		}},
		{"native_energyAtBlock", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self        common.Address
				BlockNumber uint32
			}
			env.ParseArgs(&args)
			ctx := env.BlockContext()
			if args.BlockNumber > ctx.Number {
				return []interface{}{&big.Int{}}
			}

			if ctx.Number-args.BlockNumber > meter.MaxBackTrackingBlockNumber {
				return []interface{}{&big.Int{}}
			}

			if args.BlockNumber == ctx.Number {
				env.UseGas(meter.GetBalanceGas)
				val := env.State().GetEnergy(meter.Address(args.Self))
				return []interface{}{val}
			}

			env.UseGas(meter.SloadGas)
			blockID := env.Seeker().GetID(args.BlockNumber)

			env.UseGas(meter.SloadGas)
			header := env.Seeker().GetHeader(blockID)

			env.UseGas(meter.SloadGas)
			state := env.State().Spawn(header.StateRoot())

			env.UseGas(meter.GetBalanceGas)
			val := state.GetEnergy(meter.Address(args.Self))

			return []interface{}{val}
		}},
		{"native_hasCode", func(env *xenv.Environment) []interface{} {
			var self common.Address
			env.ParseArgs(&self)

			env.UseGas(meter.GetBalanceGas)
			hasCode := !env.State().GetCodeHash(meter.Address(self)).IsZero()

			return []interface{}{hasCode}
		}},
		{"native_storageFor", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self common.Address
				Key  meter.Bytes32
			}
			env.ParseArgs(&args)

			env.UseGas(meter.SloadGas)
			storage := env.State().GetStorage(meter.Address(args.Self), args.Key)
			return []interface{}{storage}
		}},
		{"native_creditPlan", func(env *xenv.Environment) []interface{} {
			var self common.Address
			env.ParseArgs(&self)
			binding := Prototype.Native(env.State()).Bind(meter.Address(self))

			env.UseGas(meter.SloadGas)
			credit, rate := binding.CreditPlan()

			return []interface{}{credit, rate}
		}},
		{"native_setCreditPlan", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self         common.Address
				Credit       *big.Int
				RecoveryRate *big.Int
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SstoreSetGas)
			binding.SetCreditPlan(args.Credit, args.RecoveryRate)
			env.Log(creditPlanEvent, meter.Address(args.Self), nil, args.Credit, args.RecoveryRate)
			return nil
		}},
		{"native_isUser", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self common.Address
				User common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			isUser := binding.IsUser(meter.Address(args.User))

			return []interface{}{isUser}
		}},
		{"native_userCredit", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self common.Address
				User common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(2 * meter.SloadGas)
			credit := binding.UserCredit(meter.Address(args.User), env.BlockContext().Time)

			return []interface{}{credit}
		}},
		{"native_addUser", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self common.Address
				User common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			if binding.IsUser(meter.Address(args.User)) {
				return []interface{}{false}
			}

			env.UseGas(meter.SstoreSetGas)
			binding.AddUser(meter.Address(args.User), env.BlockContext().Time)

			var action meter.Bytes32
			copy(action[:], "added")
			env.Log(userEvent, meter.Address(args.Self), []meter.Bytes32{meter.BytesToBytes32(args.User[:])}, action)
			return []interface{}{true}
		}},
		{"native_removeUser", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self common.Address
				User common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			if !binding.IsUser(meter.Address(args.User)) {
				return []interface{}{false}
			}

			env.UseGas(meter.SstoreResetGas)
			binding.RemoveUser(meter.Address(args.User))

			var action meter.Bytes32
			copy(action[:], "removed")
			env.Log(userEvent, meter.Address(args.Self), []meter.Bytes32{meter.BytesToBytes32(args.User[:])}, action)
			return []interface{}{true}
		}},
		{"native_sponsor", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self    common.Address
				Sponsor common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			if binding.IsSponsor(meter.Address(args.Sponsor)) {
				return []interface{}{false}
			}

			env.UseGas(meter.SstoreSetGas)
			binding.Sponsor(meter.Address(args.Sponsor), true)

			var action meter.Bytes32
			copy(action[:], "sponsored")
			env.Log(sponsorEvent, meter.Address(args.Self), []meter.Bytes32{meter.BytesToBytes32(args.Sponsor.Bytes())}, action)
			return []interface{}{true}
		}},
		{"native_unsponsor", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self    common.Address
				Sponsor common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			if !binding.IsSponsor(meter.Address(args.Sponsor)) {
				return []interface{}{false}
			}

			env.UseGas(meter.SstoreResetGas)
			binding.Sponsor(meter.Address(args.Sponsor), false)

			var action meter.Bytes32
			copy(action[:], "unsponsored")
			env.Log(sponsorEvent, meter.Address(args.Self), []meter.Bytes32{meter.BytesToBytes32(args.Sponsor.Bytes())}, action)
			return []interface{}{true}
		}},
		{"native_isSponsor", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self    common.Address
				Sponsor common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			isSponsor := binding.IsSponsor(meter.Address(args.Sponsor))

			return []interface{}{isSponsor}
		}},
		{"native_selectSponsor", func(env *xenv.Environment) []interface{} {
			var args struct {
				Self    common.Address
				Sponsor common.Address
			}
			env.ParseArgs(&args)
			binding := Prototype.Native(env.State()).Bind(meter.Address(args.Self))

			env.UseGas(meter.SloadGas)
			if !binding.IsSponsor(meter.Address(args.Sponsor)) {
				return []interface{}{false}
			}

			env.UseGas(meter.SstoreResetGas)
			binding.SelectSponsor(meter.Address(args.Sponsor))

			var action meter.Bytes32
			copy(action[:], "selected")
			env.Log(sponsorEvent, meter.Address(args.Self), []meter.Bytes32{meter.BytesToBytes32(args.Sponsor.Bytes())}, action)

			return []interface{}{true}
		}},
		{"native_currentSponsor", func(env *xenv.Environment) []interface{} {
			var self common.Address
			env.ParseArgs(&self)
			binding := Prototype.Native(env.State()).Bind(meter.Address(self))

			env.UseGas(meter.SloadGas)
			addr := binding.CurrentSponsor()

			return []interface{}{addr}
		}},
	}
	abi := Prototype.NativeABI()
	for _, def := range defines {
		if method, found := abi.MethodByName(def.name); found {
			nativeMethods[methodKey{Prototype.Address, method.ID()}] = &nativeMethod{
				abi: method,
				run: def.run,
			}
		} else {
			panic("method not found: " + def.name)
		}
	}
}
