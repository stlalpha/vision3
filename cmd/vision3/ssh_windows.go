//go:build windows

package main

import (
	"fmt"
	"log"
)

// startSSHServer is not supported on Windows (requires libssh C library).
func startSSHServer(hostKeyPath, sshHost string, sshPort int, legacyAlgorithms bool) (sshAcceptor, func(), error) {
	log.Printf("WARN: SSH server (libssh) is not available on Windows")
	return nil, nil, fmt.Errorf("SSH server is not supported on Windows (requires libssh C library)")
}
