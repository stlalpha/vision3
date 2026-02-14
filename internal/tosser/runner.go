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
		log.Printf("INFO: Tosser[%s] polling disabled (poll_interval_seconds=0). Use RunOnce() for manual toss.", t.networkName)
		return
	}

	interval := time.Duration(t.config.PollSeconds) * time.Second
	log.Printf("INFO: Tosser[%s] started. Polling every %v.", t.networkName, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("INFO: Tosser[%s] stopping.", t.networkName)
			// Save dupe DB on shutdown
			if err := t.dupeDB.Save(); err != nil {
				log.Printf("WARN: Tosser[%s] failed to save dupe DB on shutdown: %v", t.networkName, err)
			}
			return
		case <-ticker.C:
			result := t.RunOnce()
			if result.PacketsProcessed > 0 || result.MessagesExported > 0 {
				log.Printf("INFO: Tosser[%s] cycle: imported=%d, exported=%d, dupes=%d, packets=%d",
					t.networkName, result.MessagesImported, result.MessagesExported,
					result.DupesSkipped, result.PacketsProcessed)
			}
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					log.Printf("ERROR: Tosser[%s] cycle: %s", t.networkName, e)
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
