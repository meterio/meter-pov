package accountlock

import (
	"bytes"
	"encoding/gob"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
)

// the global variables in AccountLock
var (
	AccountLockAddr       = meter.BytesToAddress([]byte("account-lock-address"))
	AccountLockProfileKey = meter.Blake2b([]byte("account-lock-profile-list-key"))
)

// Candidate List
func (a *AccountLock) GetProfileList(state *state.State) (result *ProfileList) {
	state.DecodeStorage(AccountLockAddr, AccountLockProfileKey, func(raw []byte) error {
		// fmt.Println("Loaded Raw Hex: ", hex.EncodeToString(raw))
		decoder := gob.NewDecoder(bytes.NewBuffer(raw))
		var lockList ProfileList
		err := decoder.Decode(&lockList)
		if err != nil {
			if err.Error() == "EOF" && len(raw) == 0 {
				// empty raw, do nothing
			} else {
				log.Warn("Error during decoding ProfileList, set it as an empty list", "err", err)
			}
			result = &ProfileList{}
			return nil

		}
		result = &lockList
		return nil
	})
	return
}

func (a *AccountLock) SetProfileList(lockList *ProfileList, state *state.State) {
	state.EncodeStorage(AccountLockAddr, AccountLockProfileKey, func() ([]byte, error) {
		buf := bytes.NewBuffer([]byte{})
		encoder := gob.NewEncoder(buf)
		err := encoder.Encode(lockList)
		return buf.Bytes(), err
	})
}
