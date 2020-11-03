// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/dfinlab/meter/consensus"
	"github.com/gonum/stat/combin"
)

func TestBroadcast(t *testing.T) {
	committeeSize := 50
	queue := make([]int, 1)
	queue[0] = 0
	visited := make(map[int]bool)
	for len(queue) > 0 {
		i := queue[0]
		if _, isVisited := visited[i]; isVisited {
			queue = queue[1:]
			continue
		}
		visited[i] = true
		peers := consensus.GetRelayPeers(i, committeeSize-1)
		fmt.Println("NODE ", i, " TO>", peers)
		for _, p := range peers {
			_, isVisited := visited[p]
			if !isVisited {
				queue = append(queue, p)
			}
		}
		queue = queue[1:]
	}
}

func TestGetRelay(t *testing.T) {
	committeeSize := 50

	fmt.Println("Testing GetRelayPeers with committee size: ", committeeSize)
	for k := 4; k < 5; k++ {
		fmt.Println("Testing with missing set size of ", k)
		missedComb := combin.Combinations(committeeSize, k)
		for j := range missedComb {
			missing := missedComb[j]
			missed := make(map[int]int)
			for _, m := range missing {
				missed[m] = m
			}
			// fmt.Println("Testing with missing : ", missing)
			visited := make(map[int]bool)
			queue := make([]int, 1)
			queue[0] = 0
			for len(queue) > 0 {
				i := queue[0]
				if _, isVisited := visited[i]; isVisited {
					queue = queue[1:]
					continue
				}
				visited[i] = true
				peers := consensus.GetRelayPeers(i, committeeSize-1)
				// fmt.Println("NODE ", i, " TO>", peers)
				for _, p := range peers {
					_, isVisited := visited[p]
					_, isMissed := missed[p]
					if !isVisited && !isMissed {
						queue = append(queue, p)
					}
				}
				queue = queue[1:]
			}
			undelivered := make([]string, 0)
			for i := 0; i < committeeSize; i++ {
				if _, ok := visited[i]; !ok {
					undelivered = append(undelivered, strconv.Itoa(i))
				}
			}
			if len(undelivered) > k {
				fmt.Println(fmt.Sprintf("missing %v, undelivered: %s", missing, strings.Join(undelivered, ", ")))
			}
		}
		// fmt.Println("Testing ended with window size of ", k)
		// fmt.Println("---------------------------------------------")
	}
}
