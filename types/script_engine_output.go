package types

import (
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
)

type ScriptEngineOutput struct {
	data      []byte
	transfers []*tx.Transfer
	events    []*tx.Event
}

func NewScriptEngineOutput(data []byte) *ScriptEngineOutput {
	return &ScriptEngineOutput{
		data:      data,
		transfers: make([]*tx.Transfer, 0),
		events:    make([]*tx.Event, 0),
	}
}

func (o *ScriptEngineOutput) SetData(d []byte) {
	o.data = d
}

func (o *ScriptEngineOutput) AddTransfer(sender, recipient meter.Address, amount *big.Int, token byte) {
	o.transfers = append(o.transfers, &tx.Transfer{
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Token:     token,
	})
}

func (o *ScriptEngineOutput) BatchAddTransfers(transfers []*tx.Transfer) {
	for _, t := range transfers {
		if t != nil {
			o.transfers = append(o.transfers, t)
		}
	}
}

func (o *ScriptEngineOutput) AddEvent(address meter.Address, topics []meter.Bytes32, data []byte) {
	o.events = append(o.events, &tx.Event{
		Address: address,
		Topics:  topics,
		Data:    data,
	})
}

func (o *ScriptEngineOutput) BatchAddEvents(events []*tx.Event) {
	for _, e := range events {
		if e != nil {
			o.events = append(o.events, e)
		}
	}
}

func (o *ScriptEngineOutput) GetTransfers() tx.Transfers {
	return o.transfers
}

func (o *ScriptEngineOutput) GetEvents() tx.Events {
	return o.events
}

func (o *ScriptEngineOutput) GetData() []byte {
	if o.data == nil || len(o.data) <= 0 {
		return nil
	}
	return o.data
}
