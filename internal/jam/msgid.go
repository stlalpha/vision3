package jam

import "fmt"

// GenerateMSGID creates a unique FTN-compatible MSGID using the base's
// serial counter. Format: "address hexserial" (e.g., "1:103/705 0012ab34").
func (b *Base) GenerateMSGID(origAddr string) (string, error) {
	serial, err := b.GetNextMsgSerial()
	if err != nil {
		return "", fmt.Errorf("jam: failed to get serial: %w", err)
	}
	return fmt.Sprintf("%s %08x", origAddr, serial), nil
}
