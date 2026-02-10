package tosser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// netNode is a net/node pair for SEEN-BY/PATH processing.
type netNode struct {
	Net  int
	Node int
}

// ParseSeenByLine parses a SEEN-BY or PATH line into net/node pairs.
// Format: "103/705 104/56 104/100" or "103/705 706" (implied net).
func ParseSeenByLine(line string) []netNode {
	var result []netNode
	currentNet := 0

	for _, part := range strings.Fields(line) {
		if idx := strings.Index(part, "/"); idx >= 0 {
			net, err1 := strconv.Atoi(part[:idx])
			node, err2 := strconv.Atoi(part[idx+1:])
			if err1 == nil && err2 == nil {
				currentNet = net
				result = append(result, netNode{Net: net, Node: node})
			}
		} else {
			// Implied net from previous entry
			node, err := strconv.Atoi(part)
			if err == nil && currentNet > 0 {
				result = append(result, netNode{Net: currentNet, Node: node})
			}
		}
	}

	return result
}

// FormatSeenByLine formats net/node pairs into a SEEN-BY/PATH line.
// Uses net compression (implied net for consecutive same-net entries).
func FormatSeenByLine(nodes []netNode) string {
	if len(nodes) == 0 {
		return ""
	}

	// Sort by net, then node
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Net != nodes[j].Net {
			return nodes[i].Net < nodes[j].Net
		}
		return nodes[i].Node < nodes[j].Node
	})

	var parts []string
	lastNet := -1

	for _, nn := range nodes {
		if nn.Net != lastNet {
			parts = append(parts, fmt.Sprintf("%d/%d", nn.Net, nn.Node))
			lastNet = nn.Net
		} else {
			parts = append(parts, strconv.Itoa(nn.Node))
		}
	}

	return strings.Join(parts, " ")
}

// MergeSeenBy merges existing SEEN-BY lines with a new address.
// Returns the updated SEEN-BY entries as a single combined line.
func MergeSeenBy(existing []string, newAddr string) []string {
	// Parse all existing entries
	var allNodes []netNode
	for _, line := range existing {
		allNodes = append(allNodes, ParseSeenByLine(line)...)
	}

	// Parse and add the new address
	newNodes := ParseSeenByLine(newAddr)
	for _, nn := range newNodes {
		// Check for duplicates
		found := false
		for _, existing := range allNodes {
			if existing.Net == nn.Net && existing.Node == nn.Node {
				found = true
				break
			}
		}
		if !found {
			allNodes = append(allNodes, nn)
		}
	}

	if len(allNodes) == 0 {
		return nil
	}

	return []string{FormatSeenByLine(allNodes)}
}

// AppendPath adds an address to the PATH lines.
func AppendPath(existing []string, newAddr string) []string {
	// Parse all existing
	var allNodes []netNode
	for _, line := range existing {
		allNodes = append(allNodes, ParseSeenByLine(line)...)
	}

	// Add new
	newNodes := ParseSeenByLine(newAddr)
	allNodes = append(allNodes, newNodes...)

	if len(allNodes) == 0 {
		return nil
	}

	return []string{FormatSeenByLine(allNodes)}
}
