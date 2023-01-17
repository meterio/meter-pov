package consensus

import (
	"bytes"
	sha256 "crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type consensusMsgInfo struct {
	//Msg    ConsensusMessage
	Msg       ConsensusMessage
	Peer      *ConsensusPeer
	RawData   []byte
	Signature []byte

	cache struct {
		msgHash    [32]byte
		msgHashHex string
	}
}

func newConsensusMsgInfo(msg ConsensusMessage, peer *ConsensusPeer, rawData []byte) *consensusMsgInfo {
	return &consensusMsgInfo{
		Msg:       msg,
		Peer:      peer,
		RawData:   rawData,
		Signature: msg.Header().Signature,
		cache: struct {
			msgHash    [32]byte
			msgHashHex string
		}{msgHashHex: ""},
	}
}

func (mi *consensusMsgInfo) MsgHashHex() string {
	if mi.cache.msgHashHex == "" {
		msgHash := sha256.Sum256(mi.RawData)
		msgHashHex := hex.EncodeToString(msgHash[:])[:8]
		mi.cache.msgHash = msgHash
		mi.cache.msgHashHex = msgHashHex
	}
	return mi.cache.msgHashHex

}

func (conR *ConsensusReactor) MarshalMsg(msg *ConsensusMessage) ([]byte, error) {
	rawMsg := cdc.MustMarshalBinaryBare(msg)
	if len(rawMsg) > maxMsgSize {
		conR.logger.Error("Msg exceeds max size", "rawMsg", len(rawMsg), "maxMsgSize", maxMsgSize)
		return make([]byte, 0), errors.New("Msg exceeds max size")
	}

	magicHex := hex.EncodeToString(conR.magic[:])
	myNetAddr := conR.GetMyNetAddr()
	payload := map[string]interface{}{
		"message":   hex.EncodeToString(rawMsg),
		"peer_ip":   myNetAddr.IP.String(),
		"peer_port": strconv.Itoa(int(myNetAddr.Port)),
		"magic":     magicHex,
	}

	return json.Marshal(payload)
}

func (conR *ConsensusReactor) UnmarshalMsg(data []byte) (*consensusMsgInfo, error) {
	var params map[string]string
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&params)
	if err != nil {
		fmt.Println(err)
		return nil, ErrUnrecognizedPayload
	}
	if strings.Compare(params["magic"], hex.EncodeToString(conR.magic[:])) != 0 {
		return nil, ErrMagicMismatch
	}
	peerIP := net.ParseIP(params["peer_ip"])
	peerPort, err := strconv.ParseUint(params["peer_port"], 10, 16)
	if err != nil {
		fmt.Println("Unrecognized Payload: ", err)
		return nil, ErrUnrecognizedPayload
	}
	peerName := conR.GetDelegateNameByIP(peerIP)
	peer := newConsensusPeer(peerName, peerIP, uint16(peerPort), conR.magic)
	rawMsg, err := hex.DecodeString(params["message"])
	if err != nil {
		fmt.Println("could not decode string: ", params["message"])
		return nil, ErrMalformattedMsg
	}
	msg, err := decodeMsg(rawMsg)
	if err != nil {
		fmt.Println("Malformatted Msg: ", msg)
		return nil, ErrMalformattedMsg
		// conR.logger.Error("Malformated message, error decoding", "peer", peerName, "ip", peerIP, "msg", msg, "err", err)
	}

	return newConsensusMsgInfo(msg, peer, data), nil
}
