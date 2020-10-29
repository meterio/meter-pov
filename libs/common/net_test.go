// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtocolAndAddress(t *testing.T) {

	cases := []struct {
		fullAddr string
		proto    string
		addr     string
	}{
		{
			"tcp://mydomain:80",
			"tcp",
			"mydomain:80",
		},
		{
			"mydomain:80",
			"tcp",
			"mydomain:80",
		},
		{
			"unix://mydomain:80",
			"unix",
			"mydomain:80",
		},
	}

	for _, c := range cases {
		proto, addr := ProtocolAndAddress(c.fullAddr)
		assert.Equal(t, proto, c.proto)
		assert.Equal(t, addr, c.addr)
	}
}
