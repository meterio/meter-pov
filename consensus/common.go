package consensus

import (
	"github.com/dfinlab/meter/types"
)

func NewConsensusCommonFromBlsCommon(blsCommon *BlsCommon) *types.ConsensusCommon {
	return &types.ConsensusCommon{
		PrivKey:     blsCommon.PrivKey,
		PubKey:      blsCommon.PubKey,
		System:      blsCommon.system,
		Params:      blsCommon.params,
		Pairing:     blsCommon.pairing,
		Initialized: true,
	}
}
