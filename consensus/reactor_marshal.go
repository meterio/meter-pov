package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func (r *Reactor) UnmarshalMsg(rawData []byte) (*IncomingMsg, error) {
	var params map[string]string
	err := json.NewDecoder(bytes.NewReader(rawData)).Decode(&params)
	if err != nil {
		fmt.Println(err)
		return nil, ErrUnrecognizedPayload
	}
	if strings.Compare(params["magic"], hex.EncodeToString(r.magic[:])) != 0 {
		return nil, ErrMagicMismatch
	}
	peerIP := net.ParseIP(params["peer_ip"])
	peerPort, err := strconv.ParseUint(params["peer_port"], 10, 16)
	if err != nil {
		fmt.Println("Unrecognized Payload: ", err)
		return nil, ErrUnrecognizedPayload
	}
	peerName := r.GetDelegateNameByIP(peerIP)
	peer := newConsensusPeer(peerName, peerIP, uint16(peerPort), r.magic)
	rawMsg, err := hex.DecodeString(params["message"])
	if err != nil {
		fmt.Println("could not decode string: ", params["message"])
		return nil, ErrMalformattedMsg
	}
	msg, err := decodeMsg(rawMsg)
	if err != nil {
		fmt.Println("Malformatted Msg: ", msg)
		return nil, ErrMalformattedMsg
		// r.logger.Error("Malformated message, error decoding", "peer", peerName, "ip", peerIP, "msg", msg, "err", err)
	}

	msgInfo := newIncomingMsg(msg, peer, rawData)
	return msgInfo, nil
}

func (r *Reactor) MarshalMsg(msg *ConsensusMessage) ([]byte, error) {
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		r.logger.Error("Msg exceeds max size", "rawMsg", len(rawMsg), "maxMsgSize", maxMsgSize)
		return make([]byte, 0), errors.New("Msg exceeds max size")
	}

	magicHex := hex.EncodeToString(r.magic[:])
	myNetAddr := r.GetMyNetAddr()
	payload := map[string]interface{}{
		"message":   hex.EncodeToString(rawMsg),
		"peer_ip":   myNetAddr.IP.String(),
		"peer_port": strconv.Itoa(int(myNetAddr.Port)),
		"magic":     magicHex,
	}

	return json.Marshal(payload)
}
