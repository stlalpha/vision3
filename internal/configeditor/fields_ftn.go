package configeditor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// fieldsFTNLink returns fields for editing a single FTN network configuration.
// Since FTN.Networks is a map, we use sorted keys for stable iteration.
func (m *Model) fieldsFTNLink() []fieldDef {
	keys := m.ftnNetworkKeys()
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(keys) {
		return nil
	}
	key := keys[idx]
	net := m.configs.FTN.Networks[key]
	netPtr := &net

	// Save closure to write back to map
	save := func() {
		m.configs.FTN.Networks[key] = *netPtr
	}

	return []fieldDef{
		{
			Label: "Network Name", Help: "Network identifier (e.g. FSXNET, FIDONET)", Type: ftString, Col: 3, Row: 1, Width: 30,
			Get: func() string { return key },
			Set: func(val string) error {
				val = strings.TrimSpace(val)
				if val == "" {
					return fmt.Errorf("network name cannot be empty")
				}
				if val == key {
					return nil
				}
				if _, exists := m.configs.FTN.Networks[val]; exists {
					return fmt.Errorf("network %q already exists", val)
				}
				cfg := m.configs.FTN.Networks[key]
				m.configs.FTN.Networks[val] = cfg
				delete(m.configs.FTN.Networks, key)
				// Update message areas that reference this network
				for i := range m.configs.MsgAreas {
					if m.configs.MsgAreas[i].Network == key {
						m.configs.MsgAreas[i].Network = val
					}
				}
				return nil
			},
			// AfterSet runs on the current model (not the stale captured pointer), so index
			// updates here are correctly applied before buildRecordFields is called.
			AfterSet: func(cur *Model, val string) {
				val = strings.TrimSpace(val)
				newKeys := cur.ftnNetworkKeys()
				idx := sort.SearchStrings(newKeys, val)
				if idx < len(newKeys) && newKeys[idx] == val {
					cur.recordEditIdx = idx
					cur.recordCursor = idx // keep list selection in sync for when user exits edit
				}
				cur.stayOnField = true // Stay on Network Name so user can see the updated value
			},
		},
		{
			Label: "Own Address", Help: "Your FTN address (e.g. 21:1/100)", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return netPtr.OwnAddress },
			Set: func(val string) error { netPtr.OwnAddress = val; save(); return nil },
		},
		{
			Label: "Tosser Enabled", Help: "Enable built-in echomail tosser", Type: ftYesNo, Col: 3, Row: 3, Width: 1,
			Get: func() string { return boolToYN(netPtr.InternalTosserEnabled) },
			Set: func(val string) error { netPtr.InternalTosserEnabled = ynToBool(val); save(); return nil },
		},
		{
			Label: "Poll Seconds", Help: "Seconds between inbound directory polls", Type: ftInteger, Col: 3, Row: 4, Width: 6, Min: 0, Max: 999999,
			Get: func() string { return strconv.Itoa(netPtr.PollSeconds) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				netPtr.PollSeconds = n
				save()
				return nil
			},
		},
		{
			Label: "Tearline", Help: "Tearline text appended to outgoing messages", Type: ftString, Col: 3, Row: 5, Width: 40,
			Get: func() string { return netPtr.Tearline },
			Set: func(val string) error { netPtr.Tearline = val; save(); return nil },
		},
	}
}

// fieldsFTNGlobal returns fields for editing the global FTN path and storage settings.
func (m *Model) fieldsFTNGlobal() []fieldDef {
	ftn := &m.configs.FTN
	return []fieldDef{
		{
			Label: "Dupe DB Path", Help: "Path to duplicate-message database file", Type: ftString, Col: 3, Row: 1, Width: 45,
			Get: func() string { return ftn.DupeDBPath },
			Set: func(val string) error { ftn.DupeDBPath = val; return nil },
		},
		{
			Label: "Inbound Path", Help: "Directory where binkd deposits received bundles", Type: ftString, Col: 3, Row: 2, Width: 45,
			Get: func() string { return ftn.InboundPath },
			Set: func(val string) error { ftn.InboundPath = val; return nil },
		},
		{
			Label: "Secure Inbound", Help: "Directory for authenticated inbound sessions", Type: ftString, Col: 3, Row: 3, Width: 45,
			Get: func() string { return ftn.SecureInboundPath },
			Set: func(val string) error { ftn.SecureInboundPath = val; return nil },
		},
		{
			Label: "Outbound Path", Help: "Staging directory for outbound .PKT files", Type: ftString, Col: 3, Row: 4, Width: 45,
			Get: func() string { return ftn.OutboundPath },
			Set: func(val string) error { ftn.OutboundPath = val; return nil },
		},
		{
			Label: "Binkd Outbound", Help: "Binkd outbound directory for ZIP bundles", Type: ftString, Col: 3, Row: 5, Width: 45,
			Get: func() string { return ftn.BinkdOutboundPath },
			Set: func(val string) error { ftn.BinkdOutboundPath = val; return nil },
		},
		{
			Label: "Temp Path", Help: "Temporary directory for bundle processing", Type: ftString, Col: 3, Row: 6, Width: 45,
			Get: func() string { return ftn.TempPath },
			Set: func(val string) error { ftn.TempPath = val; return nil },
		},
		{
			Label: "Bad Area Tag", Help: "Area tag for unrecognized echomail", Type: ftString, Col: 3, Row: 7, Width: 20,
			Get: func() string { return ftn.BadAreaTag },
			Set: func(val string) error { ftn.BadAreaTag = val; return nil },
		},
		{
			Label: "Dupe Area Tag", Help: "Area tag for duplicate messages", Type: ftString, Col: 3, Row: 8, Width: 20,
			Get: func() string { return ftn.DupeAreaTag },
			Set: func(val string) error { ftn.DupeAreaTag = val; return nil },
		},
	}
}

// ftnLinkRef identifies a single link by its parent network key and position in the Links slice.
type ftnLinkRef struct {
	networkKey string
	linkIdx    int
}

// ftnAllLinkRefs returns a flat, ordered list of all links across all networks.
// Ordered by sorted network key, then by link index within each network.
func (m Model) ftnAllLinkRefs() []ftnLinkRef {
	nets := m.ftnNetworkKeys()
	var refs []ftnLinkRef
	for _, k := range nets {
		net := m.configs.FTN.Networks[k]
		for i := range net.Links {
			refs = append(refs, ftnLinkRef{networkKey: k, linkIdx: i})
		}
	}
	return refs
}

// fieldsFTNLinkEdit returns fields for editing a single FTN link.
func (m *Model) fieldsFTNLinkEdit() []fieldDef {
	refs := m.ftnAllLinkRefs()
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(refs) {
		return nil
	}
	ref := refs[idx]
	netKey := ref.networkKey
	linkIdx := ref.linkIdx

	// Working copy of the link; save() writes it back through the map.
	netCfg := m.configs.FTN.Networks[netKey]
	linkCopy := netCfg.Links[linkIdx]
	linkPtr := &linkCopy

	save := func() {
		nc := m.configs.FTN.Networks[netKey]
		if linkIdx < len(nc.Links) {
			nc.Links[linkIdx] = *linkPtr
			m.configs.FTN.Networks[netKey] = nc
		}
	}

	return []fieldDef{
		{
			Label: "Network", Help: "FTN network this link belongs to", Type: ftLookup, Col: 3, Row: 1, Width: 30,
			Get: func() string { return netKey },
			Set: func(val string) error {
				if val == netKey {
					return nil
				}
				if _, exists := m.configs.FTN.Networks[val]; !exists {
					return fmt.Errorf("network %q not found", val)
				}
				// Remove link from source network
				src := m.configs.FTN.Networks[netKey]
				link := src.Links[linkIdx]
				src.Links = append(src.Links[:linkIdx], src.Links[linkIdx+1:]...)
				m.configs.FTN.Networks[netKey] = src
				// Append link to destination network
				dst := m.configs.FTN.Networks[val]
				dst.Links = append(dst.Links, link)
				m.configs.FTN.Networks[val] = dst
				return nil
			},
			AfterSet: func(cur *Model, val string) {
				// Find the link's new position in the flat list (it was appended to val network)
				newRefs := cur.ftnAllLinkRefs()
				for i, r := range newRefs {
					if r.networkKey == val {
						nc := cur.configs.FTN.Networks[r.networkKey]
						if r.linkIdx == len(nc.Links)-1 {
							cur.recordEditIdx = i
							cur.recordCursor = i
							break
						}
					}
				}
			},
			LookupItems: func() []LookupItem {
				return m.buildFTNNetworkLookupItems()
			},
		},
		{
			Label: "Address", Help: "FTN node address (e.g. 21:4/158)", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return linkPtr.Address },
			Set: func(val string) error {
				val = strings.TrimSpace(val)
				if val == "" {
					return fmt.Errorf("address cannot be empty")
				}
				linkPtr.Address = val
				save()
				return nil
			},
		},
		{
			Label: "Packet Password", Help: "Packet password for this link (max 8 chars)", Type: ftString, Col: 3, Row: 3, Width: 8,
			Get: func() string { return linkPtr.PacketPassword },
			Set: func(val string) error {
				val = strings.TrimSpace(val)
				if len(val) > 8 {
					return fmt.Errorf("packet password must be at most 8 characters")
				}
				linkPtr.PacketPassword = val
				save()
				return nil
			},
		},
		{
			Label: "Areafix Password", Help: "Password for AreaFix netmail (subject line)", Type: ftString, Col: 3, Row: 4, Width: 20,
			Get: func() string { return linkPtr.AreafixPassword },
			Set: func(val string) error { linkPtr.AreafixPassword = val; save(); return nil },
		},
		{
			Label: "Name", Help: "Descriptive name for this link (e.g. FSXNet Hub)", Type: ftString, Col: 3, Row: 5, Width: 40,
			Get: func() string { return linkPtr.Name },
			Set: func(val string) error { linkPtr.Name = val; save(); return nil },
		},
		{
			Label: "Flavour", Help: "Delivery flavour: Normal, Crash, Hold, Direct", Type: ftLookup, Col: 3, Row: 6, Width: 10,
			Get: func() string {
				if linkPtr.Flavour == "" {
					return "Normal"
				}
				return linkPtr.Flavour
			},
			Set: func(val string) error { linkPtr.Flavour = val; save(); return nil },
			LookupItems: func() []LookupItem {
				return []LookupItem{
					{Value: "Normal", Display: "Normal - standard delivery"},
					{Value: "Crash", Display: "Crash - immediate delivery"},
					{Value: "Hold", Display: "Hold - wait for poll"},
					{Value: "Direct", Display: "Direct - direct call"},
				}
			},
		},
	}
}

// buildFTNNetworkLookupItems returns lookup items for all configured FTN networks.
func (m *Model) buildFTNNetworkLookupItems() []LookupItem {
	keys := m.ftnNetworkKeys()
	items := make([]LookupItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, LookupItem{Value: k, Display: k})
	}
	return items
}

// ftnLinkCount returns the total number of links across all FTN networks.
func (m Model) ftnLinkCount() int {
	total := 0
	for _, net := range m.configs.FTN.Networks {
		total += len(net.Links)
	}
	return total
}

