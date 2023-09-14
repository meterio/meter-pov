package consensus

import (
	sha256 "crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/types"
)

const (
	IN_QUEUE_TTL = time.Second * 5
)

type IncomingMsg struct {
	//Msg    ConsensusMessage
	Msg          ConsensusMessage
	Peer         *ConsensusPeer
	RawData      []byte
	Hash         [32]byte
	ShortHashStr string

	Signer types.CommitteeMember

	EnqueueAt time.Time
	ExpireAt  time.Time
}

func newIncomingMsg(msg ConsensusMessage, peer *ConsensusPeer, rawData []byte) *IncomingMsg {
	msgHash := sha256.Sum256(rawData)
	shortMsgHash := hex.EncodeToString(msgHash[:])[:8]
	return &IncomingMsg{
		Msg:          msg,
		Peer:         peer,
		RawData:      rawData,
		Hash:         msgHash,
		ShortHashStr: shortMsgHash,
	}
}

type IncomingQueue struct {
	sync.Mutex
	logger log15.Logger
	queue  chan (*IncomingMsg)
	cache  *lru.ARCCache
}

func NewIncomingQueue() *IncomingQueue {
	cache, err := lru.NewARC(2048)
	if err != nil {
		panic("could not create cache")
	}
	return &IncomingQueue{
		logger: log15.New("pkg", "in"),
		queue:  make(chan (*IncomingMsg), 2048),
		cache:  cache,
	}
}

func (q *IncomingQueue) ForceAdd(mi *IncomingMsg) {
	defer q.Mutex.Unlock()
	q.Mutex.Lock()

	for len(q.queue) >= cap(q.queue) {
		dropped := <-q.queue
		q.logger.Warn(fmt.Sprintf("dropped %s due to cap", dropped.Msg.String()), "from", dropped.Peer.NameAndIP())
	}

	q.queue <- mi
}

func (q *IncomingQueue) Add(mi *IncomingMsg) error {
	defer q.Mutex.Unlock()
	q.Mutex.Lock()
	if q.cache.Contains(mi.Hash) {
		return ErrKnownMsg
	}
	q.cache.Add(mi.Hash, true)

	// instead of drop the latest message, drop the oldest one in front of queue
	for len(q.queue) >= cap(q.queue) {
		dropped := <-q.queue
		q.logger.Warn(fmt.Sprintf("dropped %s due to cap", dropped.Msg.String()), "from", dropped.Peer.NameAndIP())
	}

	// TODO: check if this caused a dead lock for putting message into a full channel
	q.logger.Info(fmt.Sprintf("recv %s", mi.Msg.String()), "from", mi.Peer.NameAndIP())
	mi.EnqueueAt = time.Now()
	mi.ExpireAt = time.Now().Add(IN_QUEUE_TTL)
	q.queue <- mi
	return nil
}

func (q *IncomingQueue) drain() {
	defer q.Mutex.Unlock()
	q.Mutex.Lock()
	for len(q.queue) > 0 {
		<-q.queue
	}
}

func (q *IncomingQueue) Queue() chan (*IncomingMsg) {
	return q.queue
}
