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

	// Read each .PKT header to determine its destination, then group by link.
	// linkPkts maps link address -> list of .pkt file paths destined for that link.
	linkPkts := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != ".pkt" {
			continue
		}
		pktPath := filepath.Join(stagingDir, entry.Name())

		hdr, err := ftn.ReadPacketHeaderFromFile(pktPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("read pkt header %s: %v", entry.Name(), err))
			continue
		}

		// Match the packet's destination to a configured link by net:node.
		matched := false
		for _, link := range t.config.Links {
			destAddr, err := jam.ParseAddress(link.Address)
			if err != nil {
				continue
			}
			if uint16(destAddr.Net) == hdr.DestNet && uint16(destAddr.Node) == hdr.DestNode {
				linkPkts[link.Address] = append(linkPkts[link.Address], pktPath)
				matched = true
				break
			}
		}
		if !matched {
			result.Errors = append(result.Errors, fmt.Sprintf(
				"pkt %s: no link found for dest %d/%d", entry.Name(), hdr.DestNet, hdr.DestNode))
		}
	}

	if len(linkPkts) == 0 {
		return result
	}

	// bundledPkts tracks which .pkt files were successfully packed so we only
	// remove those — leaving any pkt that failed bundling for retry next run.
	bundledPkts := make(map[string]bool)

	dayIdx := int(time.Now().Weekday()+6) % 7 // Monday=0 ... Sunday=6

	for _, link := range t.config.Links {
		pkts := linkPkts[link.Address]
		if len(pkts) == 0 {
			continue
		}

		destAddr, err := jam.ParseAddress(link.Address)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse link address %q: %v", link.Address, err))
			continue
		}

		bundleName := ftn.BundleFileName(uint16(destAddr.Net), uint16(destAddr.Node), dayIdx)
		bundlePath := filepath.Join(binkdDir, bundleName)
		bundlePath = resolveUniqueBundlePath(bundlePath)

		count, err := ftn.CreateBundle(bundlePath, pkts)
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

		// Mark these pkts as successfully bundled.
		for _, p := range pkts {
			bundledPkts[p] = true
		}

		// Create BSO flow file (.clo/.hlo) for crash/hold delivery.
		if err := writeFlowFile(binkdDir, destAddr, link.Flavour, bundlePath); err != nil {
			log.Printf("WARN: Pack: failed to write flow file for %s: %v", link.Address, err)
		}
	}

	// Remove only the .pkt files that were successfully bundled.
	for pktPath := range bundledPkts {
		if err := os.Remove(pktPath); err != nil {
			log.Printf("WARN: Pack: failed to remove staged pkt %s: %v", pktPath, err)
		}
	}

	return result
}

// writeFlowFile appends a bundle path to the BSO flow file for a link with
// crash/hold delivery flavour. For Normal delivery no flow file is needed.
// The ^ prefix instructs binkd to delete the bundle after successful transmission.
func writeFlowFile(dir string, destAddr *jam.FidoAddress, flavour, bundlePath string) error {
	var ext string
	switch strings.ToUpper(flavour) {
	case "CRASH":
		ext = ".clo"
	case "HOLD":
		ext = ".hlo"
	case "DIRECT":
		ext = ".dlo"
	default: // "NORMAL", ""
		return nil
	}

	flowName := fmt.Sprintf("%04x%04x%s", destAddr.Net, destAddr.Node, ext)
	flowPath := filepath.Join(dir, flowName)

	f, err := os.OpenFile(flowPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		absPath = bundlePath
	}
	_, err = fmt.Fprintf(f, "^%s\n", absPath)
	return err
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
