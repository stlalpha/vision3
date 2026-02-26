package configeditor

import (
	"strconv"

	"github.com/stlalpha/vision3/internal/config"
)

// buildSysFields returns field definitions for the given system config sub-screen.
func (m *Model) buildSysFields(screen int) []fieldDef {
	cfg := &m.configs.Server
	switch screen {
	case 0:
		return sysFieldsRegistration(cfg)
	case 1:
		return sysFieldsNetwork(cfg)
	case 2:
		return sysFieldsLimits(cfg)
	case 3:
		return sysFieldsLevels(cfg)
	case 4:
		return sysFieldsDefaults(cfg)
	case 5:
		return sysFieldsIPLists(cfg)
	}
	return nil
}

// sysFieldsRegistration returns fields for BBS Registration sub-screen.
func sysFieldsRegistration(cfg *config.ServerConfig) []fieldDef {
	return []fieldDef{
		{
			Label: "Board Name", Help: "Your BBS name shown to users", Type: ftString, Col: 3, Row: 1, Width: 40,
			Get: func() string { return cfg.BoardName },
			Set: func(val string) error { cfg.BoardName = val; return nil },
		},
		{
			Label: "SysOp Name", Help: "System operator name", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return cfg.SysOpName },
			Set: func(val string) error { cfg.SysOpName = val; return nil },
		},
		{
			Label: "Timezone", Help: "IANA timezone (e.g. America/Chicago)", Type: ftString, Col: 3, Row: 3, Width: 30,
			Get: func() string { return cfg.Timezone },
			Set: func(val string) error { cfg.Timezone = val; return nil },
		},
	}
}

// sysFieldsNetwork returns fields for Network Setup sub-screen.
func sysFieldsNetwork(cfg *config.ServerConfig) []fieldDef {
	return []fieldDef{
		{
			Label: "SSH Enabled", Help: "Enable SSH server", Type: ftYesNo, Col: 3, Row: 1, Width: 1,
			Get: func() string { return boolToYN(cfg.SSHEnabled) },
			Set: func(val string) error { cfg.SSHEnabled = ynToBool(val); return nil },
		},
		{
			Label: "SSH Host", Help: "Listen address (blank=all interfaces)", Type: ftString, Col: 3, Row: 2, Width: 20,
			Get: func() string { return cfg.SSHHost },
			Set: func(val string) error { cfg.SSHHost = val; return nil },
		},
		{
			Label: "SSH Port", Help: "SSH listen port (default: 8022)", Type: ftInteger, Col: 3, Row: 3, Width: 5, Min: 1, Max: 65535,
			Get: func() string { return strconv.Itoa(cfg.SSHPort) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.SSHPort = n
				return nil
			},
		},
		{
			Label: "Legacy SSH", Help: "Allow legacy algorithms for older clients", Type: ftYesNo, Col: 3, Row: 4, Width: 1,
			Get: func() string { return boolToYN(cfg.LegacySSHAlgorithms) },
			Set: func(val string) error { cfg.LegacySSHAlgorithms = ynToBool(val); return nil },
		},
		{
			Label: "Telnet Enabled", Help: "Enable Telnet server", Type: ftYesNo, Col: 3, Row: 6, Width: 1,
			Get: func() string { return boolToYN(cfg.TelnetEnabled) },
			Set: func(val string) error { cfg.TelnetEnabled = ynToBool(val); return nil },
		},
		{
			Label: "Telnet Host", Help: "Listen address (blank=all interfaces)", Type: ftString, Col: 3, Row: 7, Width: 20,
			Get: func() string { return cfg.TelnetHost },
			Set: func(val string) error { cfg.TelnetHost = val; return nil },
		},
		{
			Label: "Telnet Port", Help: "Telnet listen port (default: 8023)", Type: ftInteger, Col: 3, Row: 8, Width: 5, Min: 1, Max: 65535,
			Get: func() string { return strconv.Itoa(cfg.TelnetPort) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.TelnetPort = n
				return nil
			},
		},
	}
}

// sysFieldsLimits returns fields for Connection Limits sub-screen.
func sysFieldsLimits(cfg *config.ServerConfig) []fieldDef {
	return []fieldDef{
		{
			Label: "Max Nodes", Help: "Maximum simultaneous connections", Type: ftInteger, Col: 3, Row: 1, Width: 5, Min: 1, Max: 999,
			Get: func() string { return strconv.Itoa(cfg.MaxNodes) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.MaxNodes = n
				return nil
			},
		},
		{
			Label: "Max Per IP", Help: "Max connections from a single IP address", Type: ftInteger, Col: 3, Row: 2, Width: 5, Min: 1, Max: 999,
			Get: func() string { return strconv.Itoa(cfg.MaxConnectionsPerIP) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.MaxConnectionsPerIP = n
				return nil
			},
		},
		{
			Label: "Failed Logins", Help: "Failed attempts before lockout (0=disabled)", Type: ftInteger, Col: 3, Row: 3, Width: 5, Min: 0, Max: 100,
			Get: func() string { return strconv.Itoa(cfg.MaxFailedLogins) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.MaxFailedLogins = n
				return nil
			},
		},
		{
			Label: "Lockout Mins", Help: "Lockout duration after failed logins", Type: ftInteger, Col: 3, Row: 4, Width: 5, Min: 0, Max: 9999,
			Get: func() string { return strconv.Itoa(cfg.LockoutMinutes) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.LockoutMinutes = n
				return nil
			},
		},
		{
			Label: "Idle Timeout", Help: "Disconnect idle users after N minutes", Type: ftInteger, Col: 3, Row: 5, Width: 5, Min: 0, Max: 999,
			Get: func() string { return strconv.Itoa(cfg.SessionIdleTimeoutMinutes) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.SessionIdleTimeoutMinutes = n
				return nil
			},
		},
		{
			Label: "Xfer Timeout", Help: "File transfer timeout in minutes", Type: ftInteger, Col: 3, Row: 6, Width: 5, Min: 0, Max: 999,
			Get: func() string { return strconv.Itoa(cfg.TransferTimeoutMinutes) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.TransferTimeoutMinutes = n
				return nil
			},
		},
	}
}

// sysFieldsLevels returns fields for Access Levels sub-screen.
func sysFieldsLevels(cfg *config.ServerConfig) []fieldDef {
	return []fieldDef{
		{
			Label: "SysOp Level", Help: "Security level for full SysOp access", Type: ftInteger, Col: 3, Row: 1, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(cfg.SysOpLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.SysOpLevel = n
				return nil
			},
		},
		{
			Label: "CoSysOp Level", Help: "Security level for CoSysOp access", Type: ftInteger, Col: 3, Row: 2, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(cfg.CoSysOpLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.CoSysOpLevel = n
				return nil
			},
		},
		{
			Label: "Invisible Lvl", Help: "Level at which user is hidden from who's online", Type: ftInteger, Col: 3, Row: 3, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(cfg.InvisibleLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.InvisibleLevel = n
				return nil
			},
		},
		{
			Label: "Regular Level", Help: "Default level for validated users", Type: ftInteger, Col: 3, Row: 4, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(cfg.RegularUserLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.RegularUserLevel = n
				return nil
			},
		},
		{
			Label: "Logon Level", Help: "Minimum level required to log in", Type: ftInteger, Col: 3, Row: 5, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(cfg.LogonLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.LogonLevel = n
				return nil
			},
		},
		{
			Label: "Anonymous Lvl", Help: "Level assigned to anonymous/guest users", Type: ftInteger, Col: 3, Row: 6, Width: 3, Min: 0, Max: 255,
			Get: func() string { return strconv.Itoa(cfg.AnonymousLevel) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.AnonymousLevel = n
				return nil
			},
		},
	}
}

// sysFieldsDefaults returns fields for Default Settings sub-screen.
func sysFieldsDefaults(cfg *config.ServerConfig) []fieldDef {
	return []fieldDef{
		{
			Label: "Allow New Users", Help: "Allow new user registration", Type: ftYesNo, Col: 3, Row: 1, Width: 1,
			Get: func() string { return boolToYN(cfg.AllowNewUsers) },
			Set: func(val string) error { cfg.AllowNewUsers = ynToBool(val); return nil },
		},
		{
			Label: "File List Mode", Help: "File listing style: lightbar or classic", Type: ftString, Col: 3, Row: 2, Width: 15,
			Get: func() string { return cfg.FileListingMode },
			Set: func(val string) error { cfg.FileListingMode = val; return nil },
		},
		{
			Label: "Del User Days", Help: "Days to keep deleted user records (0=purge now, -1=forever)", Type: ftInteger, Col: 3, Row: 3, Width: 5, Min: -1, Max: 9999,
			Get: func() string { return strconv.Itoa(cfg.DeletedUserRetentionDays) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				cfg.DeletedUserRetentionDays = n
				return nil
			},
		},
	}
}

// sysFieldsIPLists returns fields for IP Blocklist/Allowlist sub-screen.
func sysFieldsIPLists(cfg *config.ServerConfig) []fieldDef {
	return []fieldDef{
		{
			Label: "Blocklist Path", Help: "Path to IP blocklist file (one IP per line)", Type: ftString, Col: 3, Row: 1, Width: 45,
			Get: func() string { return cfg.IPBlocklistPath },
			Set: func(val string) error { cfg.IPBlocklistPath = val; return nil },
		},
		{
			Label: "Allowlist Path", Help: "Path to IP allowlist file (one IP per line)", Type: ftString, Col: 3, Row: 2, Width: 45,
			Get: func() string { return cfg.IPAllowlistPath },
			Set: func(val string) error { cfg.IPAllowlistPath = val; return nil },
		},
	}
}
