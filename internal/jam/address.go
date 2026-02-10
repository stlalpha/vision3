package jam

import (
	"fmt"
	"strconv"
	"strings"
)

// FidoAddress represents a parsed FidoNet 4D address (Zone:Net/Node.Point).
type FidoAddress struct {
	Zone  int
	Net   int
	Node  int
	Point int
}

// ParseAddress parses a FidoNet address string in the format "Z:N/N" or "Z:N/N.P".
func ParseAddress(addr string) (*FidoAddress, error) {
	addr = strings.TrimSpace(addr)

	parts := strings.SplitN(addr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("jam: invalid address format: %s", addr)
	}

	zone, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jam: invalid zone: %s", parts[0])
	}

	netNode := strings.SplitN(parts[1], "/", 2)
	if len(netNode) != 2 {
		return nil, fmt.Errorf("jam: invalid net/node: %s", parts[1])
	}

	net, err := strconv.Atoi(netNode[0])
	if err != nil {
		return nil, fmt.Errorf("jam: invalid net: %s", netNode[0])
	}

	nodePoint := strings.SplitN(netNode[1], ".", 2)
	node, err := strconv.Atoi(nodePoint[0])
	if err != nil {
		return nil, fmt.Errorf("jam: invalid node: %s", nodePoint[0])
	}

	point := 0
	if len(nodePoint) == 2 {
		point, err = strconv.Atoi(nodePoint[1])
		if err != nil {
			return nil, fmt.Errorf("jam: invalid point: %s", nodePoint[1])
		}
	}

	return &FidoAddress{
		Zone:  zone,
		Net:   net,
		Node:  node,
		Point: point,
	}, nil
}

// String returns the full 4D address. Point is omitted if zero.
func (a *FidoAddress) String() string {
	if a.Point == 0 {
		return fmt.Sprintf("%d:%d/%d", a.Zone, a.Net, a.Node)
	}
	return fmt.Sprintf("%d:%d/%d.%d", a.Zone, a.Net, a.Node, a.Point)
}

// String2D returns the 2D address format (net/node) used for SEEN-BY and PATH.
func (a *FidoAddress) String2D() string {
	return fmt.Sprintf("%d/%d", a.Net, a.Node)
}
