package types_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/meterio/meter-pov/types"
)

func TestEmptyMember(t *testing.T) {
	m := types.CommitteeMember{}
	fmt.Println(m.NameWithIP())
}

func TestMember(t *testing.T) {
	m := types.CommitteeMember{Name: "test", NetAddr: *types.NewNetAddressIPPort(net.IPv4(0xf2, 0xe1, 0x22, 0x11), 1000)}
	fmt.Println(m.NameWithIP())
}
