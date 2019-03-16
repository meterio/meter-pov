package consensus

import (
	"github.com/dfinlab/go-amino"
	//"github.com/dfinlab/meter/types"
)

var cdc = amino.NewCodec()

func init() {
	RegisterConsensusMessages(cdc)
	//    RegisterWALMessages(cdc)
	//    types.RegisterBlockAmino(cdc)
}
