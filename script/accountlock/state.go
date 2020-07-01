package accountlock

import (
	"bytes"
	"strings"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/rlp"
)

// the global variables in AccountLock
var (
	//0x6163636f756e742d6c6f636b2d61646472657373
	AccountLockAddr       = meter.BytesToAddress([]byte("account-lock-address"))
	AccountLockProfileKey = meter.Blake2b([]byte("account-lock-profile-list-key"))
)

// Candidate List
func (a *AccountLock) GetProfileList(state *state.State) (result *ProfileList) {
	state.DecodeStorage(AccountLockAddr, AccountLockProfileKey, func(raw []byte) error {
		profiles := make([]*Profile, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &profiles)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					log.Warn("Error during decoding profile list", "err", err)
					return err
				}
			}
		}

		result = NewProfileList(profiles)
		return nil
	})
	return
}

func (a *AccountLock) SetProfileList(lockList *ProfileList, state *state.State) {
	/*****
	sort.SliceStable(lockList.Profiles, func(i, j int) bool {
		return bytes.Compare(lockList.Profiles[i].Addr.Bytes(), lockList.Profiles[j].Addr.Bytes()) <= 0
	})
	*****/
	state.EncodeStorage(AccountLockAddr, AccountLockProfileKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(lockList.Profiles)
	})
}
