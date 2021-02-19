// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package powpool

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/builtin"
	"github.com/dfinlab/meter/chain"
	"github.com/dfinlab/meter/co"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/state"
	"github.com/ethereum/go-ethereum/event"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	GlobPowPoolInst *PowPool

	powBlockRecvedGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pow_block_recved",
		Help: "Accumulated counter for received pow blocks since last k-block",
	})
)

// Options options for tx pool.
type Options struct {
	Node            string
	Port            int
	User            string
	Pass            string
	Limit           int
	LimitPerAccount int
	MaxLifetime     time.Duration
}

type PowReward struct {
	Rewarder meter.Address
	Value    big.Int
}

// pow decisions
type PowResult struct {
	Nonce         uint32
	Rewards       []PowReward
	Difficaulties *big.Int
	Raw           []block.PowRawBlock
}

// PowBlockEvent will be posted when pow is added or status changed.
type PowBlockEvent struct {
	BlockInfo *PowBlockInfo
}

// PowPool maintains unprocessed transactions.
type PowPool struct {
	chain        *chain.Chain
	stateCreator *state.Creator
	options      Options
	all          *powObjectMap

	done    chan struct{}
	powFeed event.Feed
	scope   event.SubscriptionScope
	goes    co.Goes
}

func SetGlobPowPoolInst(pool *PowPool) bool {
	GlobPowPoolInst = pool
	return true
}

func GetGlobPowPoolInst() *PowPool {
	return GlobPowPoolInst
}

// New create a new PowPool instance.
// Shutdown is required to be called at end.
func New(options Options, chain *chain.Chain, stateCreator *state.Creator) *PowPool {
	pool := &PowPool{
		chain:        chain,
		stateCreator: stateCreator,
		options:      options,
		all:          newPowObjectMap(),
		done:         make(chan struct{}),
	}
	pool.goes.Go(pool.housekeeping)
	SetGlobPowPoolInst(pool)
	prometheus.MustRegister(powBlockRecvedGauge)

	return pool
}

func (p *PowPool) housekeeping() {
}

// Close cleanup inner go routines.
func (p *PowPool) Close() {
	close(p.done)
	p.scope.Close()
	p.goes.Wait()
	log.Debug("closed")
}

//SubscribePowBlockEvent receivers will receive a pow
func (p *PowPool) SubscribePowBlockEvent(ch chan *PowBlockEvent) event.Subscription {
	return p.scope.Track(p.powFeed.Subscribe(ch))
}

func (p *PowPool) InitialAddKframe(newPowBlockInfo *PowBlockInfo) error {
	err := p.Wash()
	if err != nil {
		return err
	}

	powObj := NewPowObject(newPowBlockInfo)
	// XXX: disable gossip
	//p.goes.Go(func() {
	//	p.powFeed.Send(&PowBlockEvent{BlockInfo: newPowBlockInfo})
	//})

	// XXX: send block to POW
	// raw := newPowBlockInfo.Raw
	// blks := bytes.Split(raw, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	// if len(blks) == 2 {
	powHex := hex.EncodeToString(newPowBlockInfo.PowRaw)
	posHex := hex.EncodeToString(newPowBlockInfo.PosRaw)
	go p.submitPosKblock(powHex, posHex)
	// } else {
	// fmt.Println("not enough items in raw block")
	// }

	return p.all.InitialAddKframe(powObj)
}

type RPCData struct {
	Jsonrpc string   `json:"jsonrpc"`
	Id      string   `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

func (p *PowPool) submitPosKblock(powHex, posHex string) (string, string) {
	client := &http.Client{}

	data := &RPCData{
		Jsonrpc: "1.0",
		Id:      "test-id",
		Method:  "submitposkblock",
		Params:  []string{powHex, posHex},
	}
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println("could not marshal json, error:", err)
		return "", ""
	}

	url := fmt.Sprintf("http://%v:%v", p.options.Node, p.options.Port)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		fmt.Println("could not create request, error:", err)
		return "", ""
	}

	auth := fmt.Sprintf("%v:%v", p.options.User, p.options.Pass)
	authToken := base64.StdEncoding.EncodeToString([]byte(auth))

	req.Header.Add("Authorization", "Basic "+authToken)
	req.Header.Set("Content-Type", "text/plain")

	res, err := client.Do(req)
	if err != nil {
		log.Warn("Post kblock failed", "url=", url)
		return "", ""
	}

	tmp := make([]byte, 1)
	content := make([]byte, 0)
	i, err := res.Body.Read(tmp)
	for i > 0 && err == nil {
		i, err = res.Body.Read(tmp)
		content = append(content, tmp...)
	}
	return res.Status, string(content)
}

// Add add new pow block into pool.
// It's not assumed as an error if the pow to be added is already in the pool,
func (p *PowPool) Add(newPowBlockInfo *PowBlockInfo) error {
	if p.all.Contains(newPowBlockInfo.HeaderHash) {
		// pow already in the pool
		log.Debug("PowPool Add, hash already in PowPool", "hash", newPowBlockInfo.HeaderHash)
		return nil
	}

	// XXX: disable powpool gossip
	//p.goes.Go(func() {
	//	p.powFeed.Send(&PowBlockEvent{BlockInfo: newPowBlockInfo})
	//})
	powObj := NewPowObject(newPowBlockInfo)
	err := p.all.Add(powObj)

	// if parent is not genesis and it's not contained in powpool
	// fetch the block immediately in a coroutine
	// Here err is set ONLY kframe is not added (not in committee).
	// XXX: a fat chance --- the powObj is already in chain, the parent block fetch is still sent.
	if err == nil && p.all.isKframeInitialAdded() && powObj.Height() >= p.all.lastKframePowObj.Height() && !p.all.Contains(powObj.blockInfo.HashPrevBlock) {
		// go p.FetchPowBlock(powObj.Height() - uint32(1))
		p.ReplayFrom(int32(p.all.lastKframePowObj.Height()) + 1)
	}

	return err
}

// Remove removes powObj from pool by its ID.
func (p *PowPool) Remove(powID meter.Bytes32) bool {
	if p.all.Remove(powID) {
		log.Debug("pow header removed", "id", powID)
		return true
	}
	return false
}

func (p *PowPool) Wash() error {
	p.all.Flush()
	return nil
}

//==============APIs for consensus ===================
func NewPowResult(nonce uint32) *PowResult {
	return &PowResult{
		Nonce:         nonce,
		Difficaulties: big.NewInt(0),
	}
}

// consensus APIs
func (p *PowPool) GetPowDecision() (bool, *PowResult) {
	var mostDifficaultResult *PowResult = nil

	// cases can not be decided
	if !p.all.isKframeInitialAdded() {
		log.Info("GetPowDecision false: kframe is not initially added")
		return false, nil
	}
	latestHeight := p.all.GetLatestHeight()
	lastKframeHeight := p.all.lastKframePowObj.Height()
	if (latestHeight < lastKframeHeight) ||
		((latestHeight - lastKframeHeight) < meter.NPowBlockPerEpoch) {
		log.Info("GetPowDecision false", "latestHeight", latestHeight, "lastKframeHeight", lastKframeHeight)
		return false, nil
	}

	// Now have enough info to process
	for _, latestObj := range p.all.GetLatestObjects() {
		result, err := p.all.FillLatestObjChain(latestObj)
		if err != nil {
			fmt.Print(err)
			continue
		}

		if mostDifficaultResult == nil {
			mostDifficaultResult = result
		} else {
			if result.Difficaulties.Cmp(mostDifficaultResult.Difficaulties) == 1 {
				mostDifficaultResult = result
			}
		}
	}

	if mostDifficaultResult == nil {
		log.Info("GetPowDecision false: not result")
		return false, nil
	} else {
		log.Info("GetPowDecision true", "latestHeight", latestHeight, "lastKframeHeight", lastKframeHeight)
		return true, mostDifficaultResult
	}
}

// func (p *PowPool) FetchPowBlock(heights ...uint32) error {
// 	host := fmt.Sprintf("%v:%v", p.options.Node, p.options.Port)
// 	client, err := rpcclient.New(&rpcclient.ConnConfig{
// 		HTTPPostMode: true,
// 		DisableTLS:   true,
// 		Host:         host,
// 		User:         p.options.User,
// 		Pass:         p.options.Pass,
// 	}, nil)
// 	if err != nil {
// 		log.Error("error creating new btc client", "err", err)
// 		return err
// 	}
// 	for _, height := range heights {
// 		hash, err := client.GetBlockHash(int64(height))
// 		if err != nil {
// 			log.Error("error getting block hash", "err", err)
// 			continue
// 		}
// 		blk, err := client.GetBlock(hash)
// 		if err != nil {
// 			log.Error("error getting block", "err", err)
// 			continue
// 		}
// 		info := NewPowBlockInfoFromPowBlock(blk)
// 		Err := p.Add(info)
// 		if Err != nil {
// 			log.Error("add to pool failed", "err", Err)
// 			return Err
// 		}
// 	}
// 	return nil
// }

func (p *PowPool) ReplayFrom(startHeight int32) error {

	host := fmt.Sprintf("%v:%v", p.options.Node, p.options.Port)
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		HTTPPostMode: true,
		DisableTLS:   true,
		Host:         host,
		User:         p.options.User,
		Pass:         p.options.Pass,
	}, nil)
	if err != nil {
		log.Error("error creating new btc client", "err", err)
		return err
	}
	hash, err := client.GetBestBlockHash()
	if err != nil {
		log.Error("error occured during getbestblockhash", "err", err)
		return err
	}

	headerVerbose, err := client.GetBlockHeaderVerbose(hash)
	if err != nil {
		log.Error("error occured during getblockheaderverbose", "err", err)
		return err
	}
	pool := GetGlobPowPoolInst()
	height := startHeight
	for height <= headerVerbose.Height {
		hash, err := client.GetBlockHash(int64(height))
		if err != nil {
			log.Error("error getting block hash", "err", err)
			return err
		}
		blk, err := client.GetBlock(hash)
		if err != nil {
			log.Error("error getting block", "err", err)
			return err
		}
		info := NewPowBlockInfoFromPowBlock(blk)
		Err := pool.Add(info)
		if Err != nil {
			log.Error("add to pool failed", "err", Err)
			return Err
		}
		height++
	}
	return nil
}

func (pool *PowPool) GetCurCoef() (curCoef int64) {
	if pool == nil {
		panic("get globalPowPool failed")
	}
	bestBlock := pool.chain.BestBlock()
	epoch := uint64(bestBlock.GetBlockEpoch())

	state, err := pool.stateCreator.NewState(bestBlock.Header().StateRoot())
	if err != nil {
		panic("get state failed")
	}
	bigCoef := builtin.Params.Native(state).Get(meter.KeyPowPoolCoef)
	coef := bigCoef.Int64()

	// builtin parameter has uint of wei, aks, 1e18, so divide by 1e9 twice
	d := builtin.Params.Native(state).Get(meter.KeyPowPoolCoefFadeDays)
	d = d.Div(d, big.NewInt(1e09))
	fd := new(big.Float).SetInt(d)
	fadeDays, _ := fd.Float64()
	fadeDays = fadeDays / (1e09)

	// builtin fade rate
	r := builtin.Params.Native(state).Get(meter.KeyPowPoolCoefFadeRate)
	r = r.Div(r, big.NewInt(1e09))
	fr := new(big.Float).SetInt(r)
	fadeRate, _ := fr.Float64()
	fadeRate = fadeRate / (1e09)

	log.Debug("GetCurCoef", "coef", coef, "epoch", epoch, "fadeDays", fadeDays, "fadeRate", fadeRate)
	curCoef = calcPowCoef(0, epoch, coef, fadeDays, fadeRate)
	log.Info("Current Coef:", "curCoef", curCoef)
	return curCoef
}
