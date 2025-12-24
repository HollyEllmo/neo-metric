package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CommentSyncer defines the interface for syncing comments
type CommentSyncer interface {
	SyncMediaComments(ctx context.Context, mediaID, accessToken string) error
	GetMediaIDsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error)
	IncrementSyncRetryCount(ctx context.Context, mediaID string, lastError string, maxRetries int) error
	ResetSyncRetryCount(ctx context.Context, mediaID string) error
}

// PublicationAccountProvider provides account info for a publication
type PublicationAccountProvider interface {
	GetAccountIDByMediaID(ctx context.Context, mediaID string) (string, error)
}

// AccountProvider provides access token for an account
type AccountProvider interface {
	GetAccessToken(ctx context.Context, accountID string) (string, error)
}

// Scheduler handles periodic synchronization of comments
type Scheduler struct {
	syncer          CommentSyncer
	pubProvider     PublicationAccountProvider
	accountProvider AccountProvider
	interval        time.Duration
	syncAge         time.Duration // How old sync status can be before refreshing
	batchSize       int           // How many media to sync per run
	maxRetries      int           // Max retries before marking sync as permanently failed
	logger          *slog.Logger
	stopCh          chan struct{}
	cancel          context.CancelFunc // Cancel function to stop in-flight operations
	wg              sync.WaitGroup
	running         bool
	mu              sync.Mutex
}

// Config holds configuration for comment sync scheduler
type Config struct {
	Interval   time.Duration
	SyncAge    time.Duration
	BatchSize  int
	MaxRetries int
}

// New creates a new comment sync scheduler
func New(
	syncer CommentSyncer,
	pubProvider PublicationAccountProvider,
	accountProvider AccountProvider,
	cfg Config,
	logger *slog.Logger,
) *Scheduler {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.SyncAge == 0 {
		cfg.SyncAge = 10 * time.Minute
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 5
	}

	return &Scheduler{
		syncer:          syncer,
		pubProvider:     pubProvider,
		accountProvider: accountProvider,
		interval:        cfg.Interval,
		syncAge:         cfg.SyncAge,
		batchSize:       cfg.BatchSize,
		maxRetries:      cfg.MaxRetries,
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

	s.logger.Info("comment sync scheduler started", "interval", s.interval, "sync_age", s.syncAge)

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
	s.logger.Info("comment sync scheduler stopped")
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run after a short delay on start (to let the app initialize)
	select {
	case <-time.After(10 * time.Second):
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

// process syncs comments for media that need it
func (s *Scheduler) process(ctx context.Context) {
	s.logger.Debug("checking for media needing comment sync")

	mediaIDs, err := s.syncer.GetMediaIDsNeedingSync(ctx, s.syncAge, s.batchSize)
	if err != nil {
		s.logger.Error("failed to get media ids needing sync", "error", err)
		return
	}

	if len(mediaIDs) == 0 {
		s.logger.Debug("no media needs comment sync")
		return
	}

	s.logger.Info("syncing comments for media", "count", len(mediaIDs))

	for _, mediaID := range mediaIDs {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := s.syncMedia(ctx, mediaID); err != nil {
			s.logger.Error("failed to sync comments", "media_id", mediaID, "error", err)
			continue
		}
		s.logger.Debug("synced comments", "media_id", mediaID)
	}
}

// syncMedia syncs comments for a single media
func (s *Scheduler) syncMedia(ctx context.Context, mediaID string) error {
	// Get account ID for this media
	accountID, err := s.pubProvider.GetAccountIDByMediaID(ctx, mediaID)
	if err != nil {
		// Increment retry count on error
		_ = s.syncer.IncrementSyncRetryCount(ctx, mediaID, err.Error(), s.maxRetries)
		return err
	}

	// Get access token for the account
	accessToken, err := s.accountProvider.GetAccessToken(ctx, accountID)
	if err != nil {
		// Increment retry count on error
		_ = s.syncer.IncrementSyncRetryCount(ctx, mediaID, err.Error(), s.maxRetries)
		return err
	}

	// Sync comments
	err = s.syncer.SyncMediaComments(ctx, mediaID, accessToken)
	if err != nil {
		// Increment retry count on error
		_ = s.syncer.IncrementSyncRetryCount(ctx, mediaID, err.Error(), s.maxRetries)
		return err
	}

	// Reset retry count on success
	_ = s.syncer.ResetSyncRetryCount(ctx, mediaID)
	return nil
}
