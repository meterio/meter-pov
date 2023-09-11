package consensus

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/inconshreveable/log15"
)

const (
	OUT_QUEUE_TTL = time.Second * 3
	REQ_TIMEOUT   = time.Second * 4
)

type OutgoingParcel struct {
	to        *ConsensusPeer
	msg       ConsensusMessage
	rawMsg    []byte
	relay     bool
	enqueueAt time.Time
	expireAt  time.Time
}

type OutgoingQueue struct {
	logger  log15.Logger
	queue   chan (*OutgoingParcel)
	clients map[string]*http.Client
}

func NewOutgoingQueue() *OutgoingQueue {
	return &OutgoingQueue{
		logger:  log15.New("pkg", "out"),
		queue:   make(chan (*OutgoingParcel), 2048),
		clients: make(map[string]*http.Client),
	}
}

func (q *OutgoingQueue) Add(to *ConsensusPeer, msg ConsensusMessage, rawMsg []byte, relay bool) {
	for len(q.queue) >= cap(q.queue) {
		p := <-q.queue
		q.logger.Info(fmt.Sprintf(`%s msg dropped due to cap ...`, p.msg.GetType()))
	}
	q.queue <- &OutgoingParcel{to: to, msg: msg, rawMsg: rawMsg, relay: relay, enqueueAt: time.Now(), expireAt: time.Now().Add(OUT_QUEUE_TTL)}
}

func (q OutgoingQueue) Start() {
	q.logger.Info(`outgoing queue started`)
	interruptCh := make(chan os.Signal, 1)
	for {
		select {
		case <-interruptCh:
			return
		case p := <-q.queue:
			if time.Now().After(p.expireAt) {
				q.logger.Info(fmt.Sprintf(`%s msg expired, dropped ...`, p.msg.GetType()))
				continue
			}
			ipAddr := p.to.netAddr.IP.String()
			if _, known := q.clients[ipAddr]; !known {
				q.clients[ipAddr] = &http.Client{Timeout: REQ_TIMEOUT}
			}
			client := q.clients[ipAddr]
			url := "http://" + p.to.netAddr.IP.String() + ":8670/pacemaker"

			if p.relay {
				q.logger.Debug(fmt.Sprintf(`relay %s`, p.msg.GetType()), "to", p.to.NameAndIP())
			} else {
				q.logger.Info(fmt.Sprintf(`send %s`, p.msg.String()), "to", p.to.NameAndIP())

			}
			res, err := client.Post(url, "application/json", bytes.NewBuffer(p.rawMsg))
			defer res.Body.Close()
			io.Copy(ioutil.Discard, res.Body)
			// TODO: print response
			if err != nil {
				q.logger.Error(fmt.Sprintf("send msg %s failed", p.msg.GetType()), "to", p.to.NameAndIP(), "err", err)
				q.clients[ipAddr] = &http.Client{Timeout: REQ_TIMEOUT}
			}
		}
	}
}
