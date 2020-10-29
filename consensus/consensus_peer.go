// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package consensus

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dfinlab/meter/types"
	"github.com/inconshreveable/log15"
)

// Consensus Topology Peer
type ConsensusPeer struct {
	name    string
	netAddr types.NetAddress
	logger  log15.Logger
	magic   [4]byte
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
	}
}

func (peer *ConsensusPeer) sendPacemakerMsg(rawData []byte, msgSummary string, relay bool) error {
	// full size message may taker longer time (> 2s) to complete the tranport.
	var netClient = &http.Client{
		Timeout: time.Second * 4, // 2
	}

	prefix := "Send>>"
	if relay {
		prefix = "Relay>>"
	}
	peer.logger.Info(prefix+" "+msgSummary, "size", len(rawData))

	url := "http://" + peer.netAddr.IP.String() + ":8670/pacemaker"
	_, err := netClient.Post(url, "application/json", bytes.NewBuffer(rawData))
	if err != nil {
		peer.logger.Error("Failed to send message to peer", "err", err)
		return err
	}
	return nil
}

func (peer *ConsensusPeer) sendCommitteeMsg(rawData []byte, msgSummary string, relay bool) error {
	var netClient = &http.Client{
		Timeout: time.Second * 4,
	}

	prefix := "Send>>"
	if relay {
		prefix = "Relay>>"
	}
	peer.logger.Info(prefix+" "+msgSummary, "size", len(rawData))
	url := "http://" + peer.netAddr.IP.String() + ":8670/committee"
	_, err := netClient.Post(url, "application/json", bytes.NewBuffer(rawData))
	if err != nil {
		peer.logger.Error("Failed to send message to peer", "err", err)
		return err
	}

	return nil
}

func (cp *ConsensusPeer) FullString() string {
	return fmt.Sprintf("%s:%d", cp.netAddr.IP.String(), cp.netAddr.Port)
}

func (cp *ConsensusPeer) String() string {
	return cp.netAddr.IP.String()
}
