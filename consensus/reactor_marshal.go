package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/meterio/meter-pov/block"
)

func (r *Reactor) UnmarshalMsg(rawData []byte) (*IncomingMsg, error) {
	var params map[string]string
	err := json.NewDecoder(bytes.NewReader(rawData)).Decode(&params)
	if err != nil {
		r.logger.Error("json decode error", "err", err)
		return nil, ErrUnrecognizedPayload
	}
	if strings.Compare(params["magic"], hex.EncodeToString(r.magic[:])) != 0 {
		return nil, ErrMagicMismatch
	}
	peerIP := net.ParseIP(params["peer_ip"])
	// peerPort, err := strconv.ParseUint(params["peer_port"], 10, 16)
	// if err != nil {
	// 	r.logger.Error("unrecognized payload", "err", err)
	// 	return nil, ErrUnrecognizedPayload
	// }
	peerName := r.getNameByIP(peerIP)
	peer := NewConsensusPeer(peerName, peerIP.String())

	msg, err := block.DecodeMsg(params["message"])
	if err != nil {
		r.logger.Error("malformatted msg", "msg", msg, "err", err)
		return nil, ErrMalformattedMsg
	}

	msgInfo := newIncomingMsg(msg, *peer, rawData)
	return msgInfo, nil
}

func (r *Reactor) MarshalMsg(msg block.ConsensusMessage) ([]byte, error) {
	rawHex, err := block.EncodeMsg(msg)
	if err != nil {
		return make([]byte, 0), err
	}

	magicHex := hex.EncodeToString(r.magic[:])
	myNetAddr := r.GetMyNetAddr()
	payload := map[string]interface{}{
		"message":   rawHex,
		"peer_ip":   myNetAddr.IP.String(),
		"peer_port": strconv.Itoa(int(myNetAddr.Port)),
		"magic":     magicHex,
	}

	return json.Marshal(payload)
}
