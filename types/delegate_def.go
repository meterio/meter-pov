package types

import "fmt"

type DelegateDef struct {
	Name        string     `json:"name"`
	Address     string     `json:"address"`
	PubKey      string     `json:"pub_key"`
	VotingPower int64      `json:"voting_power"`
	NetAddr     NetAddress `json:"network_addr"`
}

func (d DelegateDef) String() string {
	return fmt.Sprintf("Name:%v, Address:%v, PubKey:%v, VotingPower:%v, NetAddr:%v", d.Name, d.Address, d.PubKey, d.VotingPower, d.NetAddr.String())
}
