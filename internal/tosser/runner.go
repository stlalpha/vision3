package tosser

import (
	"context"
	"log"
	"time"
)

// Start begins the tosser background polling loop.
// It runs import+export cycles at the configured interval.
// Call cancel on the context to stop.
func (t *Tosser) Start(ctx context.Context) {
	if t.config.PollSeconds <= 0 {
		log.Printf("INFO: Tosser polling disabled (poll_interval_seconds=0). Use RunOnce() for manual toss.")
		return
	}

	interval := time.Duration(t.config.PollSeconds) * time.Second
	log.Printf("INFO: Tosser started. Polling every %v.", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("INFO: Tosser stopping.")
			// Save dupe DB on shutdown
			if err := t.dupeDB.Save(); err != nil {
				log.Printf("WARN: Failed to save dupe DB on shutdown: %v", err)
			}
			return
		case <-ticker.C:
			result := t.RunOnce()
			if result.PacketsProcessed > 0 || result.MessagesExported > 0 {
				log.Printf("INFO: Toss cycle: imported=%d, exported=%d, dupes=%d, packets=%d",
					result.MessagesImported, result.MessagesExported,
					result.DupesSkipped, result.PacketsProcessed)
			}
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					log.Printf("ERROR: Toss cycle: %s", e)
				}
			}
		}
	}
}

// RunOnce performs a single import+export cycle.
func (t *Tosser) RunOnce() TossResult {
	importResult := t.ProcessInbound()
	exportResult := t.ScanAndExport()

	return TossResult{
		PacketsProcessed: importResult.PacketsProcessed,
		MessagesImported: importResult.MessagesImported,
		MessagesExported: exportResult.MessagesExported,
		DupesSkipped:     importResult.DupesSkipped,
		Errors:           append(importResult.Errors, exportResult.Errors...),
	}
}

// PurgeDupes removes old entries from the dupe database.
func (t *Tosser) PurgeDupes() error {
	return t.dupeDB.Purge()
}
