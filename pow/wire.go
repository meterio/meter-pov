package pow

import (
	"github.com/dfinlab/go-amino"
)

var cdc = amino.NewCodec()

func init() {
	RegisterPowMessages(cdc)
}
