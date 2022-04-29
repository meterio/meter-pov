// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/gonum/stat/combin"
	"github.com/meterio/meter-pov/consensus"
)

type HopRecord struct {
	index int
	hop   int
}

func TestBroadcast(t *testing.T) {
	committeeSize := 203
	queue := make([]*HopRecord, 1)
	queue[0] = &HopRecord{index: 0, hop: 0}
	visited := make(map[int]bool)

	rcvCount := make(map[int]int)
	hopCount := make(map[int]int)
	msgCount := 0
	broadcasted := false
	for len(queue) > 0 {
		node := queue[0]
		i := node.index
		hop := node.hop
		if _, isVisited := visited[i]; isVisited {
			queue = queue[1:]
			continue
		}
		visited[i] = true
		peers := consensus.GetRelayPeers(i, committeeSize)
		fmt.Println("NODE ", i, " TO>", peers)
		for _, p := range peers {
			msgCount++
			if _, ok := rcvCount[p]; !ok {
				rcvCount[p] = 0
			}
			rcvCount[p] += 1
			if _, ok := hopCount[p]; !ok {
				hopCount[p] = hop
			}
			_, isVisited := visited[p]
			if len(rcvCount) == committeeSize && !broadcasted {
				fmt.Println("Broadcasted with ", msgCount, "messages")
				broadcasted = true
			}
			if !isVisited {
				queue = append(queue, &HopRecord{index: p, hop: hop + 1})
			}
		}
		queue = queue[1:]
	}

	for i := 0; i < committeeSize; i++ {
		if _, ok := rcvCount[i]; ok {
			if _, ok := hopCount[i]; ok {
				fmt.Println("Node ", i, "recved ", rcvCount[i], " copies", "with ", hopCount[i], "hops")
			}
		} else {
			fmt.Println("Node ", i, "recved no copies")
		}
	}
}

func TestRobustness(t *testing.T) {
	committeeSize := 200
	testRound := 1000

	fmt.Println("FailNode,CommitteeSize,TestRounds,MissingTotal,Missing%,Suspend%")
	totalMissed := 0
	for failNodeCount := 1; failNodeCount < committeeSize/3; failNodeCount++ {
		suspend := 0
		for r := 0; r < testRound; r++ {
			queue := make([]int, 1)
			queue[0] = 0
			visited := make(map[int]bool)

			failed := make(map[int]bool)
			for i := 0; i < failNodeCount; i++ {
				index := rand.Intn(committeeSize - 1)
				_, had := failed[index]
				for ; had || index == 0; index = rand.Intn(committeeSize - 1) {
					_, had = failed[index]
				}
				failed[index] = true
			}

			rcvCount := make(map[int]int)
			for len(queue) > 0 {
				i := queue[0]
				if _, isVisited := visited[i]; isVisited {
					queue = queue[1:]
					continue
				}
				visited[i] = true
				if _, isFailed := failed[i]; isFailed {
					rcvCount[i] = 1
					// failed node could not send to peers anymore
					continue
				}
				peers := consensus.GetRelayPeers(i, committeeSize)
				// fmt.Println("NODE ", i, " TO>", peers)
				for _, p := range peers {
					if _, ok := rcvCount[p]; !ok {
						rcvCount[p] = 0
					}
					rcvCount[p] += 1
					_, isVisited := visited[p]
					if !isVisited {
						queue = append(queue, p)
					}
				}
				queue = queue[1:]
			}

			missed := 0
			for i := 1; i < committeeSize; i++ {
				if _, ok := rcvCount[i]; !ok {
					// fmt.Println("Node ", i, "recved ", rcvCount[i], " copies")
					// fmt.Println("Node ", i, "recved no copies")
					missed++
				}
			}
			if missed > 0 {
				// for fi, _ := range failed {
				// fmt.Println("Failed node:", fi)
				// }
				// fmt.Println("missed: ", missed)
			}
			totalMissed += missed
			if missed+failNodeCount > committeeSize/3 {
				suspend += 1
			}
		}
		missingRate := float64(totalMissed) * 100 / float64(committeeSize) / float64(testRound)
		suspendRate := float64(suspend) * 100 / float64(testRound)
		fmt.Println(fmt.Sprintf("%d,%d,%d,%d,%.3f%%,%.3f%%", failNodeCount, committeeSize, testRound, totalMissed, missingRate, suspendRate))
	}
}

func TesGetRelay(t *testing.T) {
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
				peers := consensus.GetRelayPeers(i, committeeSize)
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
