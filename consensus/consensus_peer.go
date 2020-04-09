package consensus

import (
	"bytes"
	"encoding/hex"
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

func (peer *ConsensusPeer) sendPacemakerMsg(rawData []byte, relay bool, msgSummary string) error {
	// full size message may taker longer time (> 2s) to complete the tranport.
	var netClient = &http.Client{
		Timeout: time.Second * 4, // 2
	}
	msgHashHex := hex.EncodeToString(rawData)[:MsgHashSize]
	if relay {
		peer.logger.Info(fmt.Sprintf("Relay: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	} else {
		peer.logger.Info(fmt.Sprintf("Send: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	}

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
	msgHashHex := hex.EncodeToString(rawData)[:MsgHashSize]
	if relay {
		peer.logger.Info(fmt.Sprintf("Relay: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	} else {
		peer.logger.Info(fmt.Sprintf("Send: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	}
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
