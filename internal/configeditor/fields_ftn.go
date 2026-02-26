package configeditor

import (
	"strconv"
)

// fieldsFTNLink returns fields for editing an FTN network configuration.
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
			Label: "Network Name", Help: "Network identifier (read-only)", Type: ftDisplay, Col: 3, Row: 1, Width: 30,
			Get: func() string { return key },
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
			Label: "Inbound Path", Help: "Directory for incoming mail bundles", Type: ftString, Col: 3, Row: 4, Width: 45,
			Get: func() string { return netPtr.InboundPath },
			Set: func(val string) error { netPtr.InboundPath = val; save(); return nil },
		},
		{
			Label: "Secure Inbound", Help: "Directory for secure/validated inbound", Type: ftString, Col: 3, Row: 5, Width: 45,
			Get: func() string { return netPtr.SecureInboundPath },
			Set: func(val string) error { netPtr.SecureInboundPath = val; save(); return nil },
		},
		{
			Label: "Outbound Path", Help: "Directory for outgoing mail bundles", Type: ftString, Col: 3, Row: 6, Width: 45,
			Get: func() string { return netPtr.OutboundPath },
			Set: func(val string) error { netPtr.OutboundPath = val; save(); return nil },
		},
		{
			Label: "Binkd Outbound", Help: "Binkd-style outbound directory", Type: ftString, Col: 3, Row: 7, Width: 45,
			Get: func() string { return netPtr.BinkdOutboundPath },
			Set: func(val string) error { netPtr.BinkdOutboundPath = val; save(); return nil },
		},
		{
			Label: "Temp Path", Help: "Temporary directory for processing", Type: ftString, Col: 3, Row: 8, Width: 45,
			Get: func() string { return netPtr.TempPath },
			Set: func(val string) error { netPtr.TempPath = val; save(); return nil },
		},
		{
			Label: "Poll Seconds", Help: "Seconds between inbound directory polls", Type: ftInteger, Col: 3, Row: 9, Width: 6, Min: 0, Max: 999999,
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
			Label: "Tearline", Help: "Tearline text appended to outgoing messages", Type: ftString, Col: 3, Row: 10, Width: 40,
			Get: func() string { return netPtr.Tearline },
			Set: func(val string) error { netPtr.Tearline = val; save(); return nil },
		},
		{
			Label: "Netmail Area", Help: "Message area tag for netmail", Type: ftString, Col: 3, Row: 11, Width: 20,
			Get: func() string { return netPtr.NetmailAreaTag },
			Set: func(val string) error { netPtr.NetmailAreaTag = val; save(); return nil },
		},
		{
			Label: "Bad Area Tag", Help: "Area tag for unrecognized echomail", Type: ftString, Col: 3, Row: 12, Width: 20,
			Get: func() string { return netPtr.BadAreaTag },
			Set: func(val string) error { netPtr.BadAreaTag = val; save(); return nil },
		},
		{
			Label: "Dupe Area Tag", Help: "Area tag for duplicate messages", Type: ftString, Col: 3, Row: 13, Width: 20,
			Get: func() string { return netPtr.DupeAreaTag },
			Set: func(val string) error { netPtr.DupeAreaTag = val; save(); return nil },
		},
	}
}

