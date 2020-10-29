// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package rpc

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

func randint64() (int64, error) {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

func init() {
	// required when generate call id
	seed, err := randint64()
	if err != nil {
		panic("could not get random int")
	}
	rand.Seed(seed)
}

const (
	rpcDefaultTimeout = time.Second * 10
)

var (
	errPeerDisconnected = errors.New("peer disconnected")
	errMsgTooLarge      = errors.New("msg too large")
	log                 = log15.New("pkg", "rpc")
)

// HandleFunc to handle received messages from peer.
type HandleFunc func(msg *p2p.Msg, write func(interface{})) error

// RPC defines the common pattern that peer interacts with each other.
type RPC struct {
	peer     *p2p.Peer
	rw       p2p.MsgReadWriter
	doneCh   chan struct{}
	pendings map[uint32]*resultListener
	lock     sync.Mutex
	logger   log15.Logger
	magic    [4]byte
}

// New create a new RPC instance.
func New(peer *p2p.Peer, rw p2p.MsgReadWriter, magic [4]byte) *RPC {
	dir := "outbound"
	if peer.Inbound() {
		dir = "inbound"
	}
	ctx := []interface{}{
		"peer", peer,
		"dir", dir,
	}
	return &RPC{
		peer:     peer,
		rw:       rw,
		doneCh:   make(chan struct{}),
		pendings: make(map[uint32]*resultListener),
		logger:   log.New(ctx...),
		magic:    magic,
	}
}

// Done returns a channel to indicates whether peer disconnected.
func (r *RPC) Done() <-chan struct{} {
	return r.doneCh
}

// Serve handles peer's IO loop, and dispatches calls and results.
func (r *RPC) Serve(handleFunc HandleFunc, maxMsgSize uint32) error {
	defer func() { close(r.doneCh) }()

	processMsg := func() error {
		msg, err := r.rw.ReadMsg()
		if err != nil {
			r.logger.Debug("failed to read msg", "err", err)
			return err
		}
		// ensure msg.Payload consumed
		defer msg.Discard()

		if msg.Size > maxMsgSize {
			r.logger.Debug("read message too large")
			return errMsgTooLarge
		}
		// parse first two elements, which are callID and isResult
		stream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := stream.List(); err != nil {
			r.logger.Debug("failed to decode msg", "err", err)
			return err
		}
		var (
			callID   uint32
			isResult bool
			magic    [4]byte
		)
		if err := stream.Decode(&callID); err != nil {
			r.logger.Debug("failed to decode msg call id", "err", err)
			return err
		}
		if err := stream.Decode(&isResult); err != nil {
			r.logger.Debug("failed to decode msg dir flag", "err", err)
			return err
		}
		if err := stream.Decode(&magic); err != nil {
			r.logger.Debug("failed to decode msg magic flag", "err", err)
			return err
		}
		if bytes.Compare(magic[:], r.magic[:]) != 0 {
			r.logger.Debug("ignored message due to magic mismatch", "expected", hex.EncodeToString(r.magic[:]), "actual", hex.EncodeToString(magic[:]))
			return errors.New("mismatch magic, different network")
		}

		if isResult {
			if err := r.handleResult(callID, &msg); err != nil {
				r.logger.Debug("handle result", "msg", msg.Code, "callid", callID, "err", err)
				return err
			}
		} else {
			if err := handleFunc(&msg, func(result interface{}) {
				if callID != 0 {
					err := p2p.Send(r.rw, msg.Code, &msgData{callID, true, r.magic, result})
					if err != nil {
						fmt.Println("could not send via p2p, error:", err)
					}

				}
				// here we skip result for Notify (callID == 0)
			}); err != nil {
				r.logger.Debug("handle call", "msg", msg.Code, "callid", callID, "err", err)
				return err
			}
		}
		return nil
	}

	for {
		if err := processMsg(); err != nil {
			return err
		}
	}
}

func (r *RPC) handleResult(callID uint32, msg *p2p.Msg) error {
	r.lock.Lock()
	listener, ok := r.pendings[callID]
	if ok {
		delete(r.pendings, callID)
	}
	r.lock.Unlock()

	if !ok {
		r.logger.Debug("unexpected call result", "msg", msg.Code)
		return nil
	}

	if listener.msgCode != msg.Code {
		return errors.New("msg code mismatch")
	}

	if err := listener.onResult(msg); err != nil {
		return err
	}
	return nil
}

func (r *RPC) prepareCall(msgCode uint64, onResult func(*p2p.Msg) error) uint32 {
	r.lock.Lock()
	defer r.lock.Unlock()
	for {
		id := rand.Uint32()
		if id == 0 {
			// 0 id is taken by Notify
			continue
		}
		if _, ok := r.pendings[id]; !ok {
			r.pendings[id] = &resultListener{
				msgCode,
				onResult,
			}
			return id
		}
	}
}
func (r *RPC) finalizeCall(id uint32) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.pendings, id)
}

// Notify notifies a message to the peer.
func (r *RPC) Notify(ctx context.Context, msgCode uint64, arg interface{}) error {
	return p2p.Send(r.rw, msgCode, &msgData{0, false, r.magic, arg})
}

// Call send a call to the peer and wait for result.
func (r *RPC) Call(ctx context.Context, msgCode uint64, arg interface{}, result interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, rpcDefaultTimeout)
	defer cancel()

	errCh := make(chan error, 1)
	id := r.prepareCall(msgCode, func(msg *p2p.Msg) error {
		// msg should decode here, or its payload will be discarded by msg loop
		err := msg.Decode(result)
		if err != nil {
			err = errors.WithMessage(err, "decode result")
		}
		errCh <- err
		return err
	})
	defer r.finalizeCall(id)

	if err := p2p.Send(r.rw, msgCode, &msgData{id, false, r.magic, arg}); err != nil {
		return err
	}

	select {
	case <-r.doneCh:
		return errPeerDisconnected
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
