package accountlock

import (
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/accountlock"
)

type AccountLockProfile struct {
	Addr           meter.Address `json:"address"`
	Memo           string        `json:"memo"`
	LockEpoch      uint32        `json:"lockEpoch"`
	ReleaseEpoch   uint32        `json:"releaseEpoch"`
	MeterAmount    string        `json:"meter"`
	MeterGovAmount string        `json:"meterGov"`
}

func convertProfileList(list *accountlock.ProfileList) []*AccountLockProfile {
	profileList := make([]*AccountLockProfile, 0)
	for _, s := range list.ToList() {
		profileList = append(profileList, convertProfile(&s))
	}
	return profileList
}

func convertProfile(a *accountlock.Profile) *AccountLockProfile {
	return &AccountLockProfile{
		Addr:           a.Addr,
		Memo:           string(a.Memo),
		LockEpoch:      a.LockEpoch,
		ReleaseEpoch:   a.ReleaseEpoch,
		MeterAmount:    a.MeterAmount.String(),
		MeterGovAmount: a.MeterGovAmount.String(),
	}
}
