package consensus

import (
	"bytes"
	"encoding/json"
	"net"

	"github.com/meterio/meter-pov/block"
)

type PMParcel struct {
	Raw   []byte `json:"raw"`
	IP    string `json:"ip"`
	Magic []byte `json:"magic"`
}

func (r *Reactor) UnmarshalMsg(rawData []byte) (*IncomingMsg, error) {
	parcel := PMParcel{}
	err := json.NewDecoder(bytes.NewReader(rawData)).Decode(&parcel)
	if err != nil {
		r.logger.Error("json decode error", "err", err)
		return nil, ErrUnrecognizedPayload
	}
	if !bytes.Equal(parcel.Magic, r.magic[:]) {
		return nil, ErrMagicMismatch
	}
	peerIP := net.ParseIP(parcel.IP)
	peerName := r.getNameByIP(peerIP)
	peer := NewConsensusPeer(peerName, peerIP.String())

	msg, err := block.DecodeMsg(parcel.Raw)
	if err != nil {
		r.logger.Error("malformatted msg", "msg", msg, "err", err)
		return nil, ErrMalformattedMsg
	}

	msgInfo := newIncomingMsg(msg, *peer, rawData)
	return msgInfo, nil
}

func (r *Reactor) MarshalMsg(msg block.ConsensusMessage) ([]byte, error) {
	raw, err := block.EncodeMsg(msg)
	if err != nil {
		return make([]byte, 0), err
	}

	myNetAddr := r.GetMyNetAddr()
	parcel := PMParcel{
		Raw:   raw,
		IP:    myNetAddr.IP.String(),
		Magic: r.magic[:],
	}

	return json.Marshal(parcel)
}
