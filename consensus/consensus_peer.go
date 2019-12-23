package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dfinlab/meter/types"
	"github.com/inconshreveable/log15"
)

// Consensus Topology Peer
type ConsensusPeer struct {
	netAddr types.NetAddress
	logger  log15.Logger
	magic   [4]byte
}

func newConsensusPeer(ip net.IP, port uint16, magic [4]byte) *ConsensusPeer {
	return &ConsensusPeer{
		netAddr: types.NetAddress{
			IP:   ip,
			Port: port,
		},
		logger: log15.New("pkg", "peer-"+ip.String()),
		magic:  magic,
	}
}

// TODO: remove srcNetAddr from input parameter
func (peer *ConsensusPeer) sendData(srcNetAddr types.NetAddress, typeName string, rawMsg []byte) error {
	magicHex := hex.EncodeToString(peer.magic[:])
	payload := map[string]interface{}{
		"message": hex.EncodeToString(rawMsg),
		"peer_ip": srcNetAddr.IP.String(),
		//"peer_id":   string(myNetAddr.ID),
		"peer_port": string(srcNetAddr.Port),
		"magic":     magicHex,
	}

	jsonStr, err := json.Marshal(payload)
	if err != nil {
		fmt.Errorf("Failed to marshal message dict to json string")
		return err
	}

	// full size message may taker longer time (> 2s) to complete the tranport.
	var netClient = &http.Client{
		Timeout: time.Second * 4, // 2
	}
	url := "http://" + peer.netAddr.IP.String() + ":8670/pacemaker"
	// peer.logger.Debug("Send", "data", string(jsonStr), "to", url)
	_, err = netClient.Post(url, "application/json", bytes.NewBuffer(jsonStr))
	if err != nil {
		peer.logger.Error("Failed to send message to peer", "peer", peer.String(), "err", err)
		return err
	}
	// TODO: check response to verify this action
	// peer.logger.Info("Sent consensus message to peer", "type", typeName, "size", len(rawMsg))
	return nil

}

func (cp *ConsensusPeer) FullString() string {
	return fmt.Sprintf("%s:%d", cp.netAddr.IP.String(), cp.netAddr.Port)
}

func (cp *ConsensusPeer) String() string {
	return cp.netAddr.IP.String()
}
