package scheduler

import (
	"context"
	"log"
	"sync"

	"github.com/robbiew/vision3/internal/config"
	"github.com/robfig/cron/v3"
)

// Scheduler manages scheduled event execution
type Scheduler struct {
	config         config.EventsConfig
	cron           *cron.Cron
	history        map[string]*EventHistory
	historyPath    string
	runningEvents  map[string]bool
	mu             sync.RWMutex
	concurrencySem chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewScheduler creates a new event scheduler
func NewScheduler(cfg config.EventsConfig, historyPath string) *Scheduler {
	// Set default max concurrent events if not specified
	if cfg.MaxConcurrentEvents <= 0 {
		cfg.MaxConcurrentEvents = 3
	}

	// Load history
	history, err := LoadHistory(historyPath)
	if err != nil {
		log.Printf("WARN: Failed to load event history from %s: %v", historyPath, err)
		history = make(map[string]*EventHistory)
	}

	return &Scheduler{
		config:         cfg,
		history:        history,
		historyPath:    historyPath,
		runningEvents:  make(map[string]bool),
		concurrencySem: make(chan struct{}, cfg.MaxConcurrentEvents),
	}
}

// Start begins the scheduler with the given context
func (s *Scheduler) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()

	// Initialize cron scheduler with seconds support
	s.cron = cron.New(cron.WithSeconds())

	// Schedule all enabled events
	enabledCount := 0
	for _, event := range s.config.Events {
		if !event.Enabled {
			log.Printf("DEBUG: Event '%s' (%s) is disabled, skipping", event.ID, event.Name)
			continue
		}

		if err := s.scheduleEvent(event); err != nil {
			log.Printf("ERROR: Failed to schedule event '%s' (%s): %v", event.ID, event.Name, err)
		} else {
			enabledCount++
			log.Printf("INFO: Event '%s' (%s) scheduled: %s", event.ID, event.Name, event.Schedule)
		}
	}

	if enabledCount == 0 {
		log.Printf("WARN: No enabled events to schedule")
		return
	}

	// Start the cron scheduler
	s.cron.Start()
	log.Printf("INFO: Event scheduler running with %d enabled events (max concurrent: %d)",
		enabledCount, s.config.MaxConcurrentEvents)

	// Wait for context cancellation
	<-s.ctx.Done()

	// Graceful shutdown
	log.Printf("INFO: Event scheduler stopping...")
	s.Stop()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	if s.cron != nil {
		// Stop accepting new jobs
		cronCtx := s.cron.Stop()

		// Wait for running jobs to complete
		<-cronCtx.Done()
		log.Printf("INFO: All scheduled events completed")
	}

	// Save history
	if err := SaveHistory(s.historyPath, s.history); err != nil {
		log.Printf("ERROR: Failed to save event history: %v", err)
	} else {
		log.Printf("INFO: Event history saved to %s", s.historyPath)
	}
}

// scheduleEvent registers an event with the cron scheduler
func (s *Scheduler) scheduleEvent(event config.EventConfig) error {
	// Parse and add the cron schedule
	_, err := s.cron.AddFunc(event.Schedule, func() {
		s.executeEventWithConcurrency(event)
	})
	return err
}

// executeEventWithConcurrency executes an event with concurrency control
func (s *Scheduler) executeEventWithConcurrency(event config.EventConfig) {
	// Check if event is already running
	s.mu.Lock()
	if s.runningEvents[event.ID] {
		s.mu.Unlock()
		log.Printf("WARN: Event '%s' (%s) skipped: already running", event.ID, event.Name)
		return
	}
	s.mu.Unlock()

	// Try to acquire concurrency semaphore
	select {
	case s.concurrencySem <- struct{}{}:
		// Acquired slot, proceed with execution
		defer func() { <-s.concurrencySem }()
	default:
		// At concurrency limit, skip execution
		log.Printf("WARN: Event '%s' (%s) skipped: max concurrent events reached (%d)",
			event.ID, event.Name, s.config.MaxConcurrentEvents)
		return
	}

	// Mark event as running
	s.mu.Lock()
	s.runningEvents[event.ID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.runningEvents, event.ID)
		s.mu.Unlock()
	}()

	// Execute the event
	result := s.executeEvent(s.ctx, event)

	// Update history
	s.updateHistory(result)
}

// GetHistory returns the current event history (for testing/monitoring)
func (s *Scheduler) GetHistory() map[string]*EventHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	historyCopy := make(map[string]*EventHistory)
	for k, v := range s.history {
		hCopy := *v
		historyCopy[k] = &hCopy
	}
	return historyCopy
}
