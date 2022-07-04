// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package abi

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	ethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/meterio/meter-pov/meter"
)

// Added to upgrade ethereum library from v1.9.1 to v1.9.3
func canonicalEventSignature(e *ethabi.Event) string {
	types := make([]string, len(e.Inputs))
	for i, input := range e.Inputs {
		types[i] = input.Type.String()
	}
	return fmt.Sprintf("%v(%v)", e.Name, strings.Join(types, ","))
}

// Event see abi.Event in go-ethereum.
type Event struct {
	id                 meter.Bytes32
	event              *ethabi.Event
	argsWithoutIndexed ethabi.Arguments
}

func newEvent(event *ethabi.Event) *Event {
	var argsWithoutIndexed ethabi.Arguments
	for _, arg := range event.Inputs {
		if !arg.Indexed {
			argsWithoutIndexed = append(argsWithoutIndexed, arg)
		}
	}
	canonicalID := crypto.Keccak256([]byte(canonicalEventSignature(event)))

	return &Event{
		meter.BytesToBytes32(canonicalID),
		event,
		argsWithoutIndexed,
	}
}

// ID returns event id.
func (e *Event) ID() meter.Bytes32 {
	return e.id
}

// Name returns event name.
func (e *Event) Name() string {
	return e.event.Name
}

// Encode encodes args to data.
func (e *Event) Encode(args ...interface{}) ([]byte, error) {
	return e.argsWithoutIndexed.Pack(args...)
}

// Decode decodes event data.
func (e *Event) Decode(data []byte, v interface{}) error {
	return e.argsWithoutIndexed.Unpack(v, data)
}
