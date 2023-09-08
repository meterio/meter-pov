// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/meterio/meter-pov/types"
)

// Consensus Topology Peer
type ConsensusPeer struct {
	name      string
	netAddr   types.NetAddress
	logger    log15.Logger
	magic     [4]byte
	netClient *http.Client
}

func newConsensusPeer(name string, ip net.IP, port uint16, magic [4]byte) *ConsensusPeer {
	return &ConsensusPeer{
		name: name,
		netAddr: types.NetAddress{
			IP:   ip,
			Port: port,
		},
		logger: log15.New("pkg", "peer", "peer", name, "ip", ip.String()),
		magic:  magic,
		netClient: &http.Client{
			Timeout: time.Second * 4, // 2
		},
	}
}

func (peer *ConsensusPeer) sendPacemakerMsg(rawData []byte, msgSummary string, shortHash string, relay bool) error {
	// full size message may taker longer time (> 2s) to complete the tranport.
	// split := strings.Split(msgSummary, " ")
	// name := ""
	// tail := ""
	// if len(split) > 0 {
	// 	name = split[0]
	// 	tail = strings.Join(split[1:], " ")
	// }

	// if relay {
	// 	peer.logger.Info("Relay>> "+name+" "+msgHashHex+"]", "size", len(rawData))
	// } else {
	// 	peer.logger.Info("Send>> "+name+" "+msgHashHex+" "+tail, "size", len(rawData))
	// }

	url := "http://" + peer.netAddr.IP.String() + ":8670/pacemaker"
	res, err := peer.netClient.Post(url, "application/json", bytes.NewBuffer(rawData))
	if err != nil {
		peer.logger.Error("Failed to send message to peer", "err", err)
		newNetClient := &http.Client{
			Timeout: time.Second * 4, // 2
		}
		peer.netClient = newNetClient
		return err
	}
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)
	return nil
}

func (cp *ConsensusPeer) String() string {
	return cp.netAddr.IP.String()
}

func (cp *ConsensusPeer) NameAndIP() string {
	return fmt.Sprintf("%s(%s)", cp.name, cp.netAddr.IP.String())
}
