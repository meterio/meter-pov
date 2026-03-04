package discv5

import (
	"net"
	"strings"
)

// Banlist stores blocked IP ranges.
type Banlist struct {
	nets []*net.IPNet
}

// NewBanlist builds a banlist from CIDRs and exact IPs.
func NewBanlist(entries []string) (*Banlist, error) {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			nets = append(nets, network)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, &net.ParseError{Type: "IP address/CIDR", Text: entry}
		}
		bits := 128
		if ip.To4() != nil {
			ip = ip.To4()
			bits = 32
		}
		nets = append(nets, &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(bits, bits),
		})
	}
	return &Banlist{nets: nets}, nil
}

func (b *Banlist) Contains(ip net.IP) bool {
	if b == nil {
		return false
	}
	for _, n := range b.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
