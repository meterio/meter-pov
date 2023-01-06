// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package state

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
)

// Profile List
func (s *State) GetProfileList() (result *meter.ProfileList) {
	s.DecodeStorage(meter.AccountLockModuleAddr, meter.ProfileListKey, func(raw []byte) error {
		profiles := make([]*meter.Profile, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &profiles)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding profile list", "err", err)
					return err
				}
			}
		}

		result = meter.NewProfileList(profiles)
		return nil
	})
	return
}

func (s *State) SetProfileList(lockList *meter.ProfileList) {
	/*****
	sort.SliceStable(lockList.Profiles, func(i, j int) bool {
		return bytes.Compare(lockList.Profiles[i].Addr.Bytes(), lockList.Profiles[j].Addr.Bytes()) <= 0
	})
	*****/
	s.EncodeStorage(meter.AccountLockModuleAddr, meter.ProfileListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(lockList.Profiles)
	})
}

// Auction List
func (s *State) GetAuctionCB() (result *meter.AuctionCB) {
	s.DecodeStorage(meter.AuctionModuleAddr, meter.AuctionCBKey, func(raw []byte) error {
		auctionCB := &meter.AuctionCB{}

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), auctionCB)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding auction control block", "err", err)
					return err
				}
			}
		}

		result = auctionCB
		return nil
	})
	return
}

func (s *State) SetAuctionCB(auctionCB *meter.AuctionCB) {
	s.EncodeStorage(meter.AuctionModuleAddr, meter.AuctionCBKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(auctionCB)
	})
}

// summary List
func (s *State) GetSummaryList() (result *meter.AuctionSummaryList) {
	s.DecodeStorage(meter.AuctionModuleAddr, meter.AuctionSummaryListKey, func(raw []byte) error {
		summaries := make([]*meter.AuctionSummary, 0)

		if len(strings.TrimSpace(string(raw))) >= 0 {
			err := rlp.Decode(bytes.NewReader(raw), &summaries)
			if err != nil {
				if err.Error() == "EOF" && len(raw) == 0 {
					// EOF is caused by no value, is not error case, so returns with empty slice
				} else {
					fmt.Println("Error during decoding auction summary list", "err", err)
					return err
				}
			}
		}

		result = meter.NewAuctionSummaryList(summaries)
		return nil
	})
	return
}

func (s *State) SetSummaryList(summaryList *meter.AuctionSummaryList) {
	/**** Do not need sort here, it is automatically sorted by Epoch
	sort.SliceStable(summaryList.Summaries, func(i, j int) bool {
		return bytes.Compare(summaryList.Summaries[i].AuctionID.Bytes(), summaryList.Summaries[j].AuctionID.Bytes()) <= 0
	})
	****/
	s.EncodeStorage(meter.AuctionModuleAddr, meter.AuctionSummaryListKey, func() ([]byte, error) {
		return rlp.EncodeToBytes(summaryList.Summaries)
	})
}
