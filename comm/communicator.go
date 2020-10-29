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

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/co"
	"github.com/dfinlab/meter/comm/proto"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/p2psrv"
	"github.com/dfinlab/meter/powpool"
	"github.com/dfinlab/meter/tx"
	"github.com/dfinlab/meter/txpool"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

var (
	log          = log15.New("pkg", "comm")
	GlobCommInst *Communicator
)

// Communicator communicates with remote p2p peers to exchange blocks and txs, etc.
type Communicator struct {
	chain          *chain.Chain
	txPool         *txpool.TxPool
	ctx            context.Context
	cancel         context.CancelFunc
	peerSet        *PeerSet
	syncedCh       chan struct{}
	newBlockFeed   event.Feed
	announcementCh chan *announcement
	feedScope      event.SubscriptionScope
	goes           co.Goes
	onceSynced     sync.Once

	powPool     *powpool.PowPool
	configTopic string
	syncTrigCh  chan bool

	magic [4]byte
}

func SetGlobCommInst(c *Communicator) {
	GlobCommInst = c
}

func GetGlobCommInst() *Communicator {
	return GlobCommInst
}

// New create a new Communicator instance.
func New(chain *chain.Chain, txPool *txpool.TxPool, powPool *powpool.PowPool, configTopic string, magic [4]byte) *Communicator {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Communicator{
		chain:          chain,
		txPool:         txPool,
		powPool:        powPool,
		ctx:            ctx,
		cancel:         cancel,
		peerSet:        newPeerSet(),
		syncedCh:       make(chan struct{}),
		announcementCh: make(chan *announcement),
		configTopic:    configTopic,
		syncTrigCh:     make(chan bool),
		magic:          magic,
	}

	SetGlobCommInst(c)
	return c
}

// Synced returns a channel indicates if synchronization process passed.
func (c *Communicator) Synced() <-chan struct{} {
	return c.syncedCh
}

// trigger a manual sync
func (c *Communicator) TriggerSync() {
	c.syncTrigCh <- true
}

// Sync start synchronization process.
func (c *Communicator) Sync(handler HandleBlockStream, qcHandler HandleQC) {
	const initSyncInterval = 2 * time.Second
	const syncInterval = 30 * time.Second

	c.goes.Go(func() {
		timer := time.NewTimer(0)
		defer timer.Stop()
		delay := initSyncInterval
		syncCount := 0

		shouldSynced := func() bool {
			bestQCHeight := c.chain.BestQC().QCHeight
			bestBlockHeight := c.chain.BestBlock().Header().Number()
			bestBlockTime := c.chain.BestBlock().Header().Timestamp()
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
				return
			case <-c.syncTrigCh:
				log.Info("Triggered synchronization start")

				best := c.chain.BestBlock().Header()
				// choose peer which has the head block with higher total score
				peer := c.peerSet.Slice().Find(func(peer *Peer) bool {
					_, totalScore := peer.Head()
					return totalScore >= best.TotalScore()
				})
				if peer != nil {
					log.Info("trigger sync with peer", "peer", peer.RemoteAddr().String())
					if err := c.sync(peer, best.Number(), handler, qcHandler); err != nil {
						peer.logger.Info("synchronization failed", "err", err)
					}
					log.Info("triggered synchronization done", "bestQC", c.chain.BestQC().QCHeight, "bestBlock", c.chain.BestBlock().Header().Number())
				}
				syncCount++
			case <-timer.C:
				log.Debug("synchronization start")

				best := c.chain.BestBlock().Header()
				// choose peer which has the head block with higher total score
				peer := c.peerSet.Slice().Find(func(peer *Peer) bool {
					_, totalScore := peer.Head()
					return totalScore >= best.TotalScore()
				})
				if peer == nil {
					// XXX: original setting was 3, changed to 1 for cold start
					if c.peerSet.Len() < 1 {
						log.Debug("no suitable peer to sync")
						break
					}
					// if more than 3 peers connected, we are assumed to be the best
					log.Debug("synchronization done, best assumed")
				} else {
					if err := c.sync(peer, best.Number(), handler, qcHandler); err != nil {
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
	genesisID := c.chain.GenesisBlock().Header().ID()
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

// Start start the communicator.
func (c *Communicator) Start() {
	c.goes.Go(c.txsLoop)
	c.goes.Go(c.announcementLoop)
	// XXX: disable powpool gossip
	//c.goes.Go(c.powsLoop)
}

// Stop stop the communicator.
func (c *Communicator) Stop() {
	c.cancel()
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
		peer.logger.Debug("failed to get status", "err", err)
		return
	}
	if status.GenesisBlockID != c.chain.GenesisBlock().Header().ID() {
		peer.logger.Debug("failed to handshake", "err", "genesis id mismatch")
		return
	}
	localClock := uint64(time.Now().Unix())
	remoteClock := status.SysTimestamp

	diff := localClock - remoteClock
	if localClock < remoteClock {
		diff = remoteClock - localClock
	}
	if diff > meter.BlockInterval*2 {
		peer.logger.Debug("failed to handshake", "err", "sys time diff too large")
		return
	}

	peer.UpdateHead(status.BestBlockID, status.TotalScore)
	c.peerSet.Add(peer, dir)
	peer.logger.Debug(fmt.Sprintf("peer added (%v)", c.peerSet.Len()))

	defer func() {
		c.peerSet.Remove(peer.ID())
		peer.logger.Debug(fmt.Sprintf("peer removed (%v)", c.peerSet.Len()))
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
func (c *Communicator) BroadcastBlock(blk *block.Block) {
	h := blk.Header()
	bestQC := c.chain.BestQCOrCandidate()
	log.Debug("Broadcast block and qc",
		"height", h.Number(),
		"id", h.ID(),
		"lastKblock", h.LastKBlockHeight(),
		"bestQC", bestQC.String())

	peers := c.peerSet.Slice().Filter(func(p *Peer) bool {
		return !p.IsBlockKnown(blk.Header().ID())
	})

	p := int(math.Sqrt(float64(len(peers))))
	toPropagate := peers[:p]
	toAnnounce := peers[p:]

	for _, peer := range toPropagate {
		peer := peer
		peer.MarkBlock(blk.Header().ID())
		c.goes.Go(func() {
			log.Debug("Sent BestQC/NewBlock to peer", "peer", peer.Peer.RemoteAddr().String())
			if err := proto.NotifyNewBestQC(c.ctx, peer, bestQC); err != nil {
				peer.logger.Debug("failed to broadcast new bestQC", "err", err)
			}
			if err := proto.NotifyNewBlock(c.ctx, peer, blk); err != nil {
				peer.logger.Debug("failed to broadcast new block", "err", err)
			}
		})
	}

	for _, peer := range toAnnounce {
		peer := peer
		peer.MarkBlock(blk.Header().ID())
		c.goes.Go(func() {
			log.Debug("Broadcast BestQC to peer", "peer", peer.Peer.RemoteAddr().String())
			if err := proto.NotifyNewBestQC(c.ctx, peer, bestQC); err != nil {
				peer.logger.Debug("failed to broadcast new bestQC", "err", err)
			}
			if err := proto.NotifyNewBlockID(c.ctx, peer, blk.Header().ID()); err != nil {
				peer.logger.Debug("failed to broadcast new block id", "err", err)
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
