// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package comm

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/block"
	"github.com/meterio/meter-pov/chain"
	"github.com/meterio/meter-pov/co"
	"github.com/meterio/meter-pov/comm/proto"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/p2psrv"
	"github.com/meterio/meter-pov/powpool"
	"github.com/meterio/meter-pov/tx"
	"github.com/meterio/meter-pov/txpool"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	log             = log15.New("pkg", "comm")
	peersCountGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "peers_count",
		Help: "Count of connected peers",
	})
)

// Communicator communicates with remote p2p peers to exchange blocks and txs, etc.
type Communicator struct {
	chain  *chain.Chain
	txPool *txpool.TxPool
	ctx    context.Context
	// cancel         context.CancelFunc
	peerSet        *PeerSet
	syncedCh       chan struct{}
	newBlockFeed   event.Feed
	announcementCh chan *announcement
	feedScope      event.SubscriptionScope
	goes           co.Goes
	onceSynced     sync.Once

	powPool     *powpool.PowPool
	configTopic string

	magic [4]byte
}

// New create a new Communicator instance.
func New(ctx context.Context, chain *chain.Chain, txPool *txpool.TxPool, powPool *powpool.PowPool, configTopic string, magic [4]byte) *Communicator {
	return &Communicator{
		chain:   chain,
		txPool:  txPool,
		powPool: powPool,
		ctx:     ctx,
		// cancel:         cancel,
		peerSet:        newPeerSet(),
		syncedCh:       make(chan struct{}),
		announcementCh: make(chan *announcement),
		configTopic:    configTopic,
		magic:          magic,
	}
}

// Synced returns a channel indicates if synchronization process passed.
func (c *Communicator) Synced() <-chan struct{} {
	return c.syncedCh
}

// Sync start synchronization process.
func (c *Communicator) Sync(handler HandleBlockStream) {
	const initSyncInterval = 500 * time.Millisecond
	const syncInterval = 6 * time.Second

	c.goes.Go(func() {
		timer := time.NewTimer(0)
		defer timer.Stop()
		delay := initSyncInterval
		syncCount := 0

		shouldSynced := func() bool {
			bestQCHeight := c.chain.BestQC().QCHeight
			bestBlockHeight := c.chain.BestBlock().Number()
			bestBlockTime := c.chain.BestBlock().Timestamp()
			now := uint64(time.Now().Unix())
			if bestBlockTime+meter.BlockInterval >= now && bestQCHeight == bestBlockHeight {
				return true
			}
			if syncCount > 2 {
				return true
			}
			return false
		}

		for {
			timer.Stop()
			timer = time.NewTimer(delay)
			select {
			case <-c.ctx.Done():
				log.Warn("stop communicator due to context end")
				return
			case <-timer.C:
				log.Debug("synchronization start")

				best := c.chain.BestBlock().Header()
				// choose peer which has the head block with higher total score
				peer := c.peerSet.Slice().Find(func(peer *Peer) bool {
					_, totalScore := peer.Head()
					log.Debug("compare score from peer", "myScore", best.TotalScore(), "peerScore", totalScore, "peer", peer.Node().IP())
					return totalScore >= best.TotalScore()
				})
				if peer == nil {
					// original setting was 3, changed to 1 for cold start
					if c.peerSet.Len() < 1 {
						log.Debug("no suitable peer to sync")
						break
					}
					// if more than 3 peers connected, we are assumed to be the best
					log.Debug("synchronization done, best assumed")
				} else {
					if err := c.sync(peer, best.Number(), handler); err != nil {
						peer.logger.Debug("synchronization failed", "err", err)
						break
					}
					peer.logger.Debug("synchronization done")
				}
				syncCount++

				if shouldSynced() {
					delay = syncInterval
					c.onceSynced.Do(func() {
						close(c.syncedCh)
					})
				}
			}
		}
	})
}

// Protocols returns all supported protocols.
func (c *Communicator) Protocols() []*p2psrv.Protocol {
	genesisID := c.chain.GenesisBlock().ID()
	return []*p2psrv.Protocol{
		&p2psrv.Protocol{
			Protocol: p2p.Protocol{
				Name:    proto.Name,
				Version: proto.Version,
				Length:  proto.Length,
				Run:     c.servePeer,
			},
			DiscTopic: fmt.Sprintf("%v%v%v@%x", proto.Name, proto.Version, c.configTopic, genesisID[24:]),
		}}
}

// Start the communicator.
func (c *Communicator) Start() {
	c.goes.Go(c.txsLoop)
	c.goes.Go(c.announcementLoop)
	// disable powpool gossip
	//c.goes.Go(c.powsLoop)
}

// Stop stop the communicator.
func (c *Communicator) Stop() {
	// c.cancel()
	c.feedScope.Close()
	c.goes.Wait()
}

type txsToSync struct {
	txs    tx.Transactions
	synced bool
}

func (c *Communicator) servePeer(p *p2p.Peer, rw p2p.MsgReadWriter) error {
	peer, dir := newPeer(p, rw, c.magic)
	curIP := peer.RemoteAddr().String()
	lastIndex := strings.LastIndex(curIP, ":")
	if lastIndex >= 0 {
		curIP = curIP[:lastIndex]
	}
	for _, knownPeer := range c.peerSet.Slice() {
		knownIP := knownPeer.RemoteAddr().String()
		lastIndex = strings.LastIndex(knownIP, ":")
		if lastIndex >= 0 {
			knownIP = knownIP[:lastIndex]
		}
		if knownIP == curIP {
			return errors.New("duplicate IP address: " + curIP)
		}
	}
	counter := c.peerSet.DirectionCount()
	if counter.Outbound*4 < counter.Inbound && dir == "inbound" {
		return errors.New("too much inbound from: " + curIP)
	}
	c.goes.Go(func() {
		c.runPeer(peer, dir)
	})

	var txsToSync txsToSync

	return peer.Serve(func(msg *p2p.Msg, w func(interface{})) error {
		return c.handleRPC(peer, msg, w, &txsToSync)
	}, proto.MaxMsgSize)
}

func (c *Communicator) runPeer(peer *Peer, dir string) {
	defer peer.Disconnect(p2p.DiscRequested)

	// 5sec timeout for handshake
	ctx, cancel := context.WithTimeout(c.ctx, time.Second*5)
	defer cancel()

	status, err := proto.GetStatus(ctx, peer)
	if err != nil {
		peer.logger.Error("Failed to get status", "err", err)
		return
	}
	if status.GenesisBlockID != c.chain.GenesisBlock().ID() {
		peer.logger.Error("Failed to handshake", "err", "genesis id mismatch")
		return
	}
	localClock := uint64(time.Now().Unix())
	remoteClock := status.SysTimestamp

	diff := localClock - remoteClock
	if localClock < remoteClock {
		diff = remoteClock - localClock
	}
	if diff > meter.BlockInterval*2 {
		peer.logger.Error("Failed to handshake", "err", "sys time diff too large")
		return
	}

	peer.UpdateHead(status.BestBlockID, status.TotalScore)
	c.peerSet.Add(peer, dir)
	peersCountGauge.Set(float64(c.peerSet.Len()))
	peer.logger.Info(fmt.Sprintf("peer added (%v)", c.peerSet.Len()))

	defer func() {
		c.peerSet.Remove(peer.ID())
		peersCountGauge.Set(float64(c.peerSet.Len()))
		peer.logger.Info(fmt.Sprintf("peer removed (%v)", c.peerSet.Len()))
	}()

	select {
	case <-peer.Done():
	case <-c.ctx.Done():
	case <-c.syncedCh:
		c.syncTxs(peer)
		select {
		case <-peer.Done():
		case <-c.ctx.Done():
		}
	}
}

// SubscribeBlock subscribe the event that new block received.
func (c *Communicator) SubscribeBlock(ch chan *NewBlockEvent) event.Subscription {
	return c.feedScope.Track(c.newBlockFeed.Subscribe(ch))
}

// BroadcastBlock broadcast a block to remote peers.
func (c *Communicator) BroadcastBlock(blk *block.EscortedBlock) {
	h := blk.Block.Header()
	qc := blk.EscortQC
	log.Debug("Broadcast block and qc",
		"height", h.Number(),
		"id", h.ID(),
		"lastKblock", h.LastKBlockHeight(),
		"escortQC", qc.String())

	peers := c.peerSet.Slice().Filter(func(p *Peer) bool {
		return !p.IsBlockKnown(blk.Block.ID())
	})

	p := int(math.Sqrt(float64(len(peers))))
	toPropagate := peers[:p]
	toAnnounce := peers[p:]

	for _, peer := range toPropagate {
		peer := peer
		peer.MarkBlock(blk.Block.ID())
		c.goes.Go(func() {
			log.Info(fmt.Sprintf("propagate %s to %s", blk.Block.ShortID(), meter.Addr2IP(peer.RemoteAddr())))
			if err := proto.NotifyNewBlock(c.ctx, peer, blk); err != nil {
				peer.logger.Error(fmt.Sprintf("Failed to propagate %s", blk.Block.ShortID()), "err", err)
			}
		})
	}

	for _, peer := range toAnnounce {
		peer := peer
		peer.MarkBlock(blk.Block.ID())
		c.goes.Go(func() {
			log.Info(fmt.Sprintf("announce %s to %s", blk.Block.ShortID(), meter.Addr2IP(peer.RemoteAddr())))
			if err := proto.NotifyNewBlockID(c.ctx, peer, blk.Block.ID()); err != nil {
				peer.logger.Error(fmt.Sprintf("Failed to announce %s", blk.Block.ShortID()), "err", err)
			}
		})
	}
}

// PeerCount returns count of peers.
func (c *Communicator) PeerCount() int {
	return c.peerSet.Len()
}

// PeersStats returns all peers' stats
func (c *Communicator) PeersStats() []*PeerStats {
	var stats []*PeerStats
	for _, peer := range c.peerSet.Slice() {
		bestID, totalScore := peer.Head()
		stats = append(stats, &PeerStats{
			Name:        peer.Name(),
			BestBlockID: bestID,
			TotalScore:  totalScore,
			PeerID:      peer.ID().String(),
			NetAddr:     peer.RemoteAddr().String(),
			Inbound:     peer.Inbound(),
			Duration:    uint64(time.Duration(peer.Duration()) / time.Second),
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Duration < stats[j].Duration
	})
	return stats
}

func (c *Communicator) PowPoolLen() int {
	return c.powPool.Len()
}
