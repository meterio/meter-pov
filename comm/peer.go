// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package comm

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/p2psrv/rpc"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	lru "github.com/hashicorp/golang-lru"
	"github.com/inconshreveable/log15"
)

const (
	maxKnownTxs       = 32768 // Maximum transactions IDs to keep in the known list (prevent DOS)
	maxKnownBlocks    = 1024  // Maximum block IDs to keep in the known list (prevent DOS)
	maxKnownPowBlocks = 1024
)

func randint64() (int64, error) {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

func init() {
	seed, err := randint64()
	if err != nil {
		panic("could not get random int")
	}
	rand.Seed(seed)
}

// Peer extends p2p.Peer with RPC integrated.
type Peer struct {
	*p2p.Peer
	*rpc.RPC
	logger log15.Logger

	createdTime    mclock.AbsTime
	knownTxs       *lru.Cache
	knownBlocks    *lru.Cache
	knownPowBlocks *lru.Cache
	head           struct {
		sync.Mutex
		id         meter.Bytes32
		totalScore uint64
	}
}

func newPeer(peer *p2p.Peer, rw p2p.MsgReadWriter, magic [4]byte) (*Peer, string) {
	dir := "outbound"
	if peer.Inbound() {
		dir = "inbound"
	}
	ctx := []interface{}{
		"peer", peer,
		"dir", dir,
	}
	knownTxs, err := lru.New(maxKnownTxs)
	if err != nil {
		fmt.Println("known tx init error:", err)
	}

	knownBlocks, err := lru.New(maxKnownBlocks)
	if err != nil {
		fmt.Println("known blocks init error:", err)
	}

	knownPowBlocks, err := lru.New(maxKnownPowBlocks)
	if err != nil {
		fmt.Println("known pow blocks init error:", err)
	}

	return &Peer{
		Peer:           peer,
		RPC:            rpc.New(peer, rw, magic),
		logger:         log.New(ctx...),
		createdTime:    mclock.Now(),
		knownTxs:       knownTxs,
		knownBlocks:    knownBlocks,
		knownPowBlocks: knownPowBlocks,
	}, dir
}

// Head returns head block ID and total score.
func (p *Peer) Head() (id meter.Bytes32, totalScore uint64) {
	p.head.Lock()
	defer p.head.Unlock()
	return p.head.id, p.head.totalScore
}

// UpdateHead update ID and total score of head block.
func (p *Peer) UpdateHead(id meter.Bytes32, totalScore uint64) {
	p.head.Lock()
	defer p.head.Unlock()
	if totalScore > p.head.totalScore {
		p.head.id, p.head.totalScore = id, totalScore
	}
}

// MarkTransaction marks a transaction to known.
func (p *Peer) MarkTransaction(id meter.Bytes32) {
	p.knownTxs.Add(id, struct{}{})
}

func (p *Peer) MarkPowBlock(id meter.Bytes32) {
	p.knownPowBlocks.Add(id, struct{}{})
}

// MarkBlock marks a block to known.
func (p *Peer) MarkBlock(id meter.Bytes32) {
	p.knownBlocks.Add(id, struct{}{})
}

// IsTransactionKnown returns if the transaction is known.
func (p *Peer) IsTransactionKnown(id meter.Bytes32) bool {
	return p.knownTxs.Contains(id)
}

func (p *Peer) IsPowBlockKnown(id meter.Bytes32) bool {
	return p.knownPowBlocks.Contains(id)
}

// IsBlockKnown returns if the block is known.
func (p *Peer) IsBlockKnown(id meter.Bytes32) bool {
	return p.knownBlocks.Contains(id)
}

// Duration returns duration of connection.
func (p *Peer) Duration() mclock.AbsTime {
	return mclock.Now() - p.createdTime
}

func (p *Peer) String() string {
	return fmt.Sprintf("%s(%d)", p.head.id.String(), p.head.totalScore)
}

// Peers slice of peers
type Peers []*Peer

// Filter filter out sub set of peers that satisfies the given condition.
func (ps Peers) Filter(cond func(*Peer) bool) Peers {
	ret := make(Peers, 0, len(ps))
	for _, peer := range ps {
		if cond(peer) {
			ret = append(ret, peer)
		}
	}
	return ret
}

// Find find one peer that satisfies the given condition.
func (ps Peers) Find(cond func(*Peer) bool) *Peer {
	for _, peer := range ps {
		if cond(peer) {
			return peer
		}
	}
	return nil
}

type DirectionCount struct {
	Inbound  int
	Outbound int
}

// PeerSet manages a set of peers, which mapped by NodeID.
type PeerSet struct {
	m       map[discover.NodeID]*Peer
	d       map[discover.NodeID]string
	counter DirectionCount
	lock    sync.Mutex
}

// NewSet create a peer set instance.
func newPeerSet() *PeerSet {
	return &PeerSet{
		m:       make(map[discover.NodeID]*Peer),
		d:       make(map[discover.NodeID]string),
		counter: DirectionCount{0, 0},
	}
}

// Add add a new peer.
func (ps *PeerSet) Add(peer *Peer, dir string) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	ps.m[peer.ID()] = peer
	ps.d[peer.ID()] = dir
	if dir == "inbound" {
		ps.counter.Inbound++
	} else {
		ps.counter.Outbound++
	}
}

// Find find peer for given nodeID.
func (ps *PeerSet) Find(nodeID discover.NodeID) *Peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return ps.m[nodeID]
}

// Remove removes peer for given nodeID.
func (ps *PeerSet) Remove(nodeID discover.NodeID) *Peer {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	if dir, ok := ps.d[nodeID]; ok {
		delete(ps.d, nodeID)
		if dir == "inbound" {
			ps.counter.Inbound--
		} else {
			ps.counter.Outbound--
		}
	}

	if peer, ok := ps.m[nodeID]; ok {
		delete(ps.m, nodeID)
		return peer
	}
	return nil
}

// Slice dumps all peers into a slice.
// The dumped slice is a random permutation.
func (ps *PeerSet) Slice() Peers {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	ret := make(Peers, len(ps.m))
	perm := rand.Perm(len(ps.m))
	i := 0
	for _, s := range ps.m {
		// randomly
		ret[perm[i]] = s
		i++
	}
	return ret
}

// Len returns length of set.
func (ps *PeerSet) Len() int {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	return len(ps.m)
}

func (ps *PeerSet) DirectionCount() DirectionCount {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return ps.counter
}
