package tosser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/ftn"
	"github.com/stlalpha/vision3/internal/jam"
)

// PackResult holds the results of a pack (bundle creation) operation.
type PackResult struct {
	BundlesCreated int
	PacketsPacked  int
	Errors         []string
}

// PackOutbound collects .PKT files from the outbound staging directory and
// creates ZIP bundle archives in the binkd outbound directory, one bundle
// per destination link. The bundle is named using the BSO convention:
//
//	NNNNFFFF.DOW0  (destNet, destNode in hex, day-of-week suffix)
//
// After successful bundling, the staged .PKT files are removed.
func (t *Tosser) PackOutbound() PackResult {
	result := PackResult{}

	stagingDir := t.config.OutboundPath
	binkdDir := t.config.BinkdOutboundPath
	if stagingDir == "" || binkdDir == "" {
		result.Errors = append(result.Errors, "outbound_path or binkd_outbound_path not configured")
		return result
	}

	// Collect all .PKT files in the staging directory
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result // Nothing to pack
		}
		result.Errors = append(result.Errors, fmt.Sprintf("read staging dir: %v", err))
		return result
	}

	var pktFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) == ".pkt" {
			pktFiles = append(pktFiles, filepath.Join(stagingDir, entry.Name()))
		}
	}

	if len(pktFiles) == 0 {
		return result // Nothing to pack
	}

	// Group .PKT files by destination link (one bundle per link)
	// Each .PKT file is addressed to a specific link — read the packet header
	// to determine the destination. For simplicity in this implementation,
	// we create one bundle per link configuration and include all staged packets.
	// A more sophisticated implementation would parse each packet header.
	for _, link := range t.config.Links {
		destAddr, err := jam.ParseAddress(link.Address)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse link address %q: %v", link.Address, err))
			continue
		}

		dayIdx := int(time.Now().Weekday()+6) % 7 // Monday=0 ... Sunday=6
		bundleName := ftn.BundleFileName(uint16(destAddr.Net), uint16(destAddr.Node), dayIdx)
		bundlePath := filepath.Join(binkdDir, bundleName)

		// If a bundle for today already exists, find a non-colliding name
		bundlePath = resolveUniqueBundlePath(bundlePath)

		count, err := ftn.CreateBundle(bundlePath, pktFiles)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create bundle for %s: %v", link.Address, err))
			continue
		}
		if count == 0 {
			continue
		}

		log.Printf("INFO: Pack: created bundle %s with %d packets for link %s",
			filepath.Base(bundlePath), count, link.Address)
		result.BundlesCreated++
		result.PacketsPacked += count
	}

	// Remove staged .PKT files after successful bundling
	if result.BundlesCreated > 0 {
		for _, pkt := range pktFiles {
			if err := os.Remove(pkt); err != nil {
				log.Printf("WARN: Pack: failed to remove staged pkt %s: %v", pkt, err)
			}
		}
	}

	return result
}

// resolveUniqueBundlePath returns a path that does not already exist. If the
// base path is free it is returned as-is; otherwise a numeric suffix is tried
// (e.g., .mo0 → .mo1 → .mo2 … up to .mo9).
func resolveUniqueBundlePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	// Try incrementing the last digit of the extension
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	prefix := ext[:len(ext)-1] // e.g., ".mo"
	for i := 1; i <= 9; i++ {
		candidate := fmt.Sprintf("%s%s%d", base, prefix, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	// Fall back to timestamp-based name
	return fmt.Sprintf("%s_%d.pkt", base, time.Now().UnixNano()&0xFFFF)
}
