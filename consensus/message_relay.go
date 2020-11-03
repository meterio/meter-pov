// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"fmt"
)

// indexes starts from 0
// 1st layer: 				0  (proposer)
// 2nd layer: 				[1, 2], [3, 4], [5, 6], [7, 8]
// 3rd layer (32 groups):   [9..] ...
func GetRelayPeers(myIndex, maxIndex int) (peers []int) {
	peers = []int{}
	if myIndex > maxIndex {
		fmt.Println("Input wrong!!! myIndex > myIndex")
		return
	}

	if myIndex == 0 {
		var k int
		if maxIndex >= 8 {
			k = 8
		} else {
			k = maxIndex
		}
		for i := 1; i <= k; i++ {
			peers = append(peers, i)
		}
		return
	}
	if maxIndex <= 8 {
		return //no peer
	}

	var groupSize, groupCount int
	groupSize = ((maxIndex - 8) / 16) + 1
	groupCount = (maxIndex - 8) / groupSize
	// fmt.Println("groupSize", groupSize, "groupCount", groupCount)

	if myIndex <= 8 {
		mySet := (myIndex - 1) / 4
		myRole := (myIndex - 1) % 4
		for i := 0; i < 8; i++ {
			group := (mySet * 8) + i
			if group >= groupCount {
				return
			}

			begin := 9 + (group * groupSize)
			if myRole == 0 {
				peers = append(peers, begin)
			} else {
				end := begin + groupSize - 1
				if end > maxIndex {
					end = maxIndex
				}
				middle := (begin + end) / 2
				peers = append(peers, middle)
			}
		}
	} else {
		// I am in group, so begin << myIndex << end
		// if wrap happens, redundant the 2nd layer
		group := (maxIndex - 8) / 16
		begin := 9 + (group * groupSize)
		end := begin + groupSize - 1
		if end > maxIndex {
			end = maxIndex
		}

		var peerIndex int
		var wrap bool = false
		if myIndex == end && end != begin {
			peers = append(peers, begin)
		}
		if peerIndex = myIndex + 1; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		} else {
			wrap = true
		}
		if peerIndex = myIndex + 2; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		} else {
			wrap = true
		}
		if peerIndex = myIndex + 4; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		} else {
			wrap = true
		}
		if peerIndex = myIndex + 8; peerIndex <= maxIndex {
			peers = append(peers, peerIndex)
		} else {
			wrap = true
		}
		if wrap == true {
			peers = append(peers, (myIndex%8)+1)
			peers = append(peers, (myIndex%8)+1+8)
		}
	}
	return
}
