package consensus

import (
	"bytes"
	sha256 "crypto/sha256"
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
	url := "http://" + peer.netAddr.IP.String() + ":8670/pacemaker"
	_, err := netClient.Post(url, "application/json", bytes.NewBuffer(rawData))
	if err != nil {
		peer.logger.Error("Failed to send message to peer", "err", err)
		return err
	}
	msgHash := sha256.Sum256(rawData)
	msgHashHex := hex.EncodeToString(msgHash[:])[:MsgHashSize]
	if relay {
		peer.logger.Info(fmt.Sprintf("Relay to peer: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	} else {
		peer.logger.Info(fmt.Sprintf("Sent to peer: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	}
	return nil
}

func (peer *ConsensusPeer) sendCommitteeMsg(rawData []byte, msgSummary string, relay bool) error {
	var netClient = &http.Client{
		Timeout: time.Second * 4,
	}
	url := "http://" + peer.netAddr.IP.String() + ":8670/committee"
	_, err := netClient.Post(url, "application/json", bytes.NewBuffer(rawData))
	if err != nil {
		peer.logger.Error("Failed to send message to peer", "err", err)
		return err
	}
	msgHash := sha256.Sum256(rawData)
	msgHashHex := hex.EncodeToString(msgHash[:])[:MsgHashSize]
	if relay {
		peer.logger.Info(fmt.Sprintf("Relay to peer: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	} else {
		peer.logger.Info(fmt.Sprintf("Sent to peer: %s", msgSummary), "size", len(rawData), "msgHash", msgHashHex)
	}
	return nil
}

func (cp *ConsensusPeer) FullString() string {
	return fmt.Sprintf("%s:%d", cp.netAddr.IP.String(), cp.netAddr.Port)
}

func (cp *ConsensusPeer) String() string {
	return cp.netAddr.IP.String()
}
