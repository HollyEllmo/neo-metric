package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// DirectSyncer defines the interface for syncing conversations
type DirectSyncer interface {
	SyncConversations(ctx context.Context, accountID, userID, accessToken string) error
	GetAccountsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error)
}

// AccountProvider provides access token and user ID for an account
type AccountProvider interface {
	GetAccessToken(ctx context.Context, accountID string) (string, error)
	GetInstagramUserID(ctx context.Context, accountID string) (string, error)
}

// Scheduler handles periodic synchronization of conversations
type Scheduler struct {
	syncer          DirectSyncer
	accountProvider AccountProvider
	interval        time.Duration
	syncAge         time.Duration // How old sync status can be before refreshing
	batchSize       int           // How many accounts to sync per run
	logger          *slog.Logger
	stopCh          chan struct{}
	cancel          context.CancelFunc // Cancel function to stop in-flight operations
	wg              sync.WaitGroup
	running         bool
	mu              sync.Mutex
}

// Config holds configuration for direct sync scheduler
type Config struct {
	Interval  time.Duration
	SyncAge   time.Duration
	BatchSize int
}

// New creates a new direct sync scheduler
func New(
	syncer DirectSyncer,
	accountProvider AccountProvider,
	cfg Config,
	logger *slog.Logger,
) *Scheduler {
	if cfg.Interval == 0 {
		cfg.Interval = 10 * time.Minute
	}
	if cfg.SyncAge == 0 {
		cfg.SyncAge = 30 * time.Minute
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 5
	}

	return &Scheduler{
		syncer:          syncer,
		accountProvider: accountProvider,
		interval:        cfg.Interval,
		syncAge:         cfg.SyncAge,
		batchSize:       cfg.BatchSize,
		logger:          logger,
		stopCh:          make(chan struct{}),
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

	// Create a cancellable context for in-flight operations
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	s.logger.Info("direct sync scheduler started", "interval", s.interval, "sync_age", s.syncAge)

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
	cancel := s.cancel
	s.mu.Unlock()

	// Cancel in-flight operations (HTTP requests, etc.)
	if cancel != nil {
		cancel()
	}

	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("direct sync scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run after a short delay on start (to let the app initialize)
	select {
	case <-time.After(15 * time.Second):
		s.process(ctx)
	case <-s.stopCh:
		return
	case <-ctx.Done():
		return
	}

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

// process syncs conversations for accounts that need it
func (s *Scheduler) process(ctx context.Context) {
	s.logger.Debug("checking for accounts needing DM sync")

	accountIDs, err := s.syncer.GetAccountsNeedingSync(ctx, s.syncAge, s.batchSize)
	if err != nil {
		s.logger.Error("failed to get accounts needing sync", "error", err)
		return
	}

	if len(accountIDs) == 0 {
		s.logger.Debug("no accounts need DM sync")
		return
	}

	s.logger.Info("syncing conversations for accounts", "count", len(accountIDs))

	for _, accountID := range accountIDs {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.syncAccount(ctx, accountID); err != nil {
			s.logger.Error("failed to sync conversations", "account_id", accountID, "error", err)
			continue
		}
		s.logger.Debug("synced conversations", "account_id", accountID)
	}
}

// syncAccount syncs conversations for a single account
func (s *Scheduler) syncAccount(ctx context.Context, accountID string) error {
	// Get access token for the account
	accessToken, err := s.accountProvider.GetAccessToken(ctx, accountID)
	if err != nil {
		return err
	}

	// Get Instagram user ID
	userID, err := s.accountProvider.GetInstagramUserID(ctx, accountID)
	if err != nil {
		return err
	}

	// Sync conversations
	return s.syncer.SyncConversations(ctx, accountID, userID, accessToken)
}
