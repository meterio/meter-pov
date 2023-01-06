// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package accountlock

import (
	"sort"

	"github.com/meterio/meter-pov/meter"
)

type AccountLockProfile struct {
	Addr           meter.Address `json:"address"`
	Memo           string        `json:"memo"`
	LockEpoch      uint32        `json:"lockEpoch"`
	ReleaseEpoch   uint32        `json:"releaseEpoch"`
	MeterAmount    string        `json:"meter"`
	MeterGovAmount string        `json:"meterGov"`
}

func convertProfileList(list *meter.ProfileList) []*AccountLockProfile {
	profileList := make([]*AccountLockProfile, 0)
	for _, s := range list.ToList() {
		profileList = append(profileList, convertProfile(&s))
	}

	// sort with descendent total points
	sort.SliceStable(profileList, func(i, j int) bool {
		return (profileList[i].ReleaseEpoch <= profileList[j].ReleaseEpoch)
	})
	return profileList
}

func convertProfile(a *meter.Profile) *AccountLockProfile {
	return &AccountLockProfile{
		Addr:           a.Addr,
		Memo:           string(a.Memo),
		LockEpoch:      a.LockEpoch,
		ReleaseEpoch:   a.ReleaseEpoch,
		MeterAmount:    a.MeterAmount.String(),
		MeterGovAmount: a.MeterGovAmount.String(),
	}
}
