package accountlock

import "github.com/dfinlab/meter/meter"

const (
	OP_ADDLOCK    = uint32(1)
	OP_REMOVELOCK = uint32(2)
	OP_TRANSFER   = uint32(3)
	OP_GOVERNING  = uint32(100)
)

// the global variables in AccountLock
var (
	//0x6163636f756e742d6c6f636b2d61646472657373
	AccountLockAddr       = meter.BytesToAddress([]byte("account-lock-address"))
	AccountLockProfileKey = meter.Blake2b([]byte("account-lock-profile-list-key"))
)
