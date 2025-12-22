package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ScheduledPublicationProcessor defines the interface for processing scheduled publications
type ScheduledPublicationProcessor interface {
	ProcessScheduledPublications(ctx context.Context) error
}

// Scheduler handles periodic processing of scheduled publications
type Scheduler struct {
	processor ScheduledPublicationProcessor
	interval  time.Duration
	logger    *slog.Logger
	stopCh    chan struct{}
	wg        sync.WaitGroup
	running   bool
	mu        sync.Mutex
}

// New creates a new scheduler
func New(processor ScheduledPublicationProcessor, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		processor: processor,
		interval:  interval,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("publication scheduler started", "interval", s.interval)

	s.wg.Add(1)
	go s.run(ctx)
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("publication scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start
	s.process(ctx)

	for {
		select {
		case <-ticker.C:
			s.process(ctx)
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// process runs the scheduled publication processor
func (s *Scheduler) process(ctx context.Context) {
	s.logger.Debug("processing scheduled publications")

	if err := s.processor.ProcessScheduledPublications(ctx); err != nil {
		s.logger.Error("failed to process scheduled publications", "error", err)
	}
}
