package script

import (
	//"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/script/staking"
	//"github.com/dfinlab/meter/state"
	//"github.com/ethereum/go-ethereum/rlp"
	//"github.com/inconshreveable/log15"
)

const (
	STAKING_MODULE_NAME = string("staking")
	STAKING_MODULE_ID   = uint32(1000)
)

func ModuleStakingInit(se *ScriptEngine) *staking.Staking {
	stk := staking.NewStaking(se.chain, se.stateCreator)
	if stk == nil {
		panic("init staking module failed")
	}

	mod := &Module{
		modName:    STAKING_MODULE_NAME,
		modID:      STAKING_MODULE_ID,
		modHandler: staking.StakingHandler,
	}
	if err := se.modReg.Register(STAKING_MODULE_ID, mod); err != nil {
		panic("register staking module failed")
	}

	stk.Start()
	se.logger.Info("ScriptEngine", "started moudle", mod.modName)
	return stk
}
